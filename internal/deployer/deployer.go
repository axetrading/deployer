package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type deployer struct {
	inputs     *Inputs
	s3Client   *s3.Client
	nextLogURL string
}

func createDeployer(inputs *Inputs) *deployer {
	return &deployer{
		inputs:     inputs,
		nextLogURL: inputs.LogURL,
	}
}

const commandsPath = "/control/commands"
const outputPath = "/control/output"
const releasePath = "/control/release"
const runPath = "/control/run"
const tfvarsPath = "/control/release/terraform/tfvars.json"

func Main(ctx context.Context, inputs *Inputs) error {
	deployer := createDeployer(inputs)
	if deployer.s3Client == nil {
		sdkConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(inputs.Region))
		if err != nil {
			return err
		}
		deployer.s3Client = s3.NewFromConfig(sdkConfig)
	}
	return deployer.run(ctx)
}

func (d *deployer) run(ctx context.Context) error {
	if err := d.initControlVolume(); err != nil {
		return err
	}
	defer func() {
		if err := d.done(); err != nil {
			log.Println("failed to mark done", err)
		}
	}()

	if err := d.downloadTerraformRelease(ctx); err != nil {
		return fmt.Errorf("failed to download terraform release: %v", err)
	}

	if err := d.terraformInit(); err != nil {
		return fmt.Errorf("failed to init terraform: %v", err)
	}

	if err := d.writeTFVars(); err != nil {
		return fmt.Errorf("failed to write tfvars: %v", err)
	}

	if d.inputs.Operation == "plan" {
		if err := d.streamCommand("terraform-plan", []string{
			"terraform",
			"plan",
			"-var-file=tfvars.json",
		}, false); err != nil {
			return err
		}
	} else if d.inputs.Operation == "apply" {
		if err := d.streamCommand("terraform-plan", []string{
			"terraform",
			"plan",
			"-var-file=tfvars.json",
			"-out=tfplan",
		}, false); err != nil {
			return err
		}
		if err := d.streamCommand("terraform-apply", []string{
			"terraform",
			"apply",
			"tfplan",
		}, false); err != nil {
			return err
		}
	}

	d.sendLogData(&linesResult{lines: []string{"done"}}, true)

	return nil
}

func (d *deployer) initControlVolume() error {
	// volume should be empty, but useful when testing locally
	if err := os.RemoveAll(commandsPath); err != nil {
		return fmt.Errorf("failed to remove commands directory: %v", err)
	}
	if err := os.RemoveAll(outputPath); err != nil {
		return fmt.Errorf("failed to remove output directory: %v", err)
	}
	if err := os.RemoveAll(releasePath); err != nil {
		return fmt.Errorf("failed to remove release directory: %v", err)
	}
	os.Remove(runPath)
	os.Remove(tfvarsPath)

	if err := os.Mkdir(commandsPath, 0777); err != nil {
		return fmt.Errorf("failed to create commands directory: %v", err)
	}
	if err := os.Mkdir(outputPath, 0777); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := os.Mkdir(releasePath, 0777); err != nil {
		return fmt.Errorf("failed to create release directory: %v", err)
	}

	if err := copy("/deployer-run", runPath); err != nil {
		return fmt.Errorf("failed to copy /run: %v", err)
	}
	if err := makeExecutable(runPath); err != nil {
		return fmt.Errorf("failed to make run executable: %v", err)
	}
	// the executable should now be exec'd in the Terraform container
	return nil
}

func (d *deployer) terraformInit() error {
	stateExists, location, err := d.stateExists()
	if err != nil {
		return err
	}
	if d.inputs.NewState && stateExists {
		return fmt.Errorf("expected to create new terraform state, but state already exists (%s)", location)
	} else if !d.inputs.NewState && !stateExists {
		return fmt.Errorf("expected to use existing terraform state, but state does not exist (%s)", location)
	}

	if err := d.streamCommand("terraform-init", []string{
		"terraform",
		"init",
		"-reconfigure",
		"-backend-config=bucket=" + d.inputs.TFStateBucket,
		"-backend-config=dynamodb_table=" + d.inputs.TFLocksTable,
		"-backend-config=workspace_key_prefix=" + d.inputs.ServiceName,
		"-backend-config=key=terraform.tfstate",
		"-backend-config=region=" + d.inputs.Region,
	}, false); err != nil {
		return err
	}

	if err := d.streamCommand("select workspace", []string{
		"terraform workspace select " + d.inputs.Workspace + " || terraform workspace new " + d.inputs.Workspace,
	}, true); err != nil {
		return err
	}

	return nil
}

func (d *deployer) writeTFVars() error {
	tfVarsFile, err := os.Create(tfvarsPath)
	if err != nil {
		return err
	}
	defer tfVarsFile.Close()
	tfVarsJSON, err := json.Marshal(d.inputs.TFVars)
	if err != nil {
		return err
	}
	if _, err := tfVarsFile.Write(tfVarsJSON); err != nil {
		return err
	}
	return nil
}

func (d *deployer) stateExists() (bool, string, error) {
	key := d.inputs.ServiceName + "/" + d.inputs.Workspace + "/terraform.tfstate"
	path := "s3://" + d.inputs.TFStateBucket + "/" + key
	if _, err := d.s3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: &d.inputs.TFStateBucket,
		Key:    &key,
	}); err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && apiError.ErrorCode() == "NotFound" {
			return false, path, nil
		}
		return false, path, err
	}
	return true, path, nil
}

//func apply(ctx context.Context, inputs *Inputs) error {
//
//}

func (d *deployer) downloadTerraformRelease(ctx context.Context) error {
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(sdkConfig)
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &d.inputs.ReleaseBucket,
		Key:    &d.inputs.ReleaseKey,
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()
	terraformReleaseFilename := "/tmp/terraform-release.zip"
	terraformReleaseFile, err := os.Create(terraformReleaseFilename)
	if err != nil {
		return err
	}
	defer terraformReleaseFile.Close()
	_, err = io.Copy(terraformReleaseFile, result.Body)
	if err != nil {
		return err
	}
	if err := unzip(terraformReleaseFilename, releasePath); err != nil {
		return err
	}
	return nil
}

func (d *deployer) streamCommand(name string, command []string, shell bool) error {
	log.Println("running command:\n\n", "    "+strings.Join(command, " ")+"\n")
	if shell {
		command = []string{"/bin/sh", "-c", command[0]}
	}
	resultChan, err := d.runCommand(name, command)
	if err != nil {
		return err
	}
	for linesResult := range resultChan {
		if err := d.sendLogData(linesResult, false); err != nil {
			return err
		}
	}
	return nil
}

func (d *deployer) runCommandSimple(name string, command []string) (string, int16, error) {
	resultChan, err := d.runCommand(name, command)
	if err != nil {
		return "", -1, err
	}
	output := ""
	for linesResult := range resultChan {
		for _, line := range linesResult.lines {
			output += line + "\n"
		}
		if linesResult.err != nil {
			if exitError, ok := linesResult.err.(*exec.ExitError); ok {
				return output, int16(exitError.ExitCode()), nil
			}
			return output, -1, linesResult.err
		}
	}
	return output, 0, nil
}

func (d *deployer) runCommand(name string, command []string) (chan *linesResult, error) {
	socket, err := net.Listen("unix", "/control/output/"+name)
	if err != nil {
		return nil, err
	}
	commandFile, err := os.Create("/control/commands/" + name)
	if err != nil {
		return nil, err
	}
	commandJSON, err := json.Marshal(command)
	if err != nil {
		return nil, err
	}
	commandFile.Write(commandJSON)
	commandFile.Close()

	conn, err := socket.Accept()
	if err != nil {
		return nil, err
	}
	linesChan := streamLineGroups(conn)
	resultChan := make(chan *linesResult)
	go func() {
		defer close(resultChan)
		defer conn.Close()
		defer socket.Close()
		defer os.Remove("/control/output/" + name)
		for linesResult := range linesChan {
			resultChan <- linesResult
			if linesResult.err != nil {
				return
			}
		}
		statusFilename := "/control/output/" + name + ".status"
		waitForFileToExist(statusFilename)
		statusBytes, err := os.ReadFile(statusFilename)
		if err != nil {
			resultChan <- &linesResult{err: err}
			return
		}
		statusInt, err := strconv.ParseUint(string(statusBytes), 10, 8)
		if err != nil {
			resultChan <- &linesResult{err: fmt.Errorf("failed to parse status: %w", err)}
			return
		}
		if statusInt != 0 {
			resultChan <- &linesResult{err: fmt.Errorf("non-zero status: %d", statusInt)}
			return
		}

	}()
	return resultChan, nil
}

func (d *deployer) sendLogData(result *linesResult, done bool) error {
	for _, line := range result.lines {
		log.Println(line)
	}
	if d.nextLogURL == "" {
		return nil
	}
	jsonData := createLinesLogData(result, done)
	nextBackoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second
	backoff := func() {
		log.Println("backing off for " + nextBackoff.String() + " before retrying")
		time.Sleep(nextBackoff)
		nextBackoff *= 2
		if nextBackoff > maxBackoff {
			nextBackoff = maxBackoff
		}
	}
	for {
		resp, err := http.Post(d.nextLogURL, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			log.Println("failed to send log data: " + err.Error())
			backoff()
			continue
		}
		if resp.StatusCode != 200 {
			if resp.StatusCode < 500 {
				log.Fatalf("unexpected client error status code from log endpoint (%s): %d\n", d.nextLogURL, resp.StatusCode)
			}
			log.Println("server error from log endpoint:", resp.StatusCode)
			backoff()
			continue
		}
		if result.err != nil {
			return result.err
		}
		jsonData, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln("failed to read response from log endpoint: " + err.Error())
		}
		var response logResposne
		if err := json.Unmarshal(jsonData, &response); err != nil {
			log.Fatalln("failed to unmarshal response from log endpoint: " + err.Error())
		}
		d.nextLogURL = response.ContinueURL
		return nil
	}
}

func (d *deployer) done() error {
	file, err := os.Create("/control/done")
	if err != nil {
		return err
	}
	return file.Close()
}
