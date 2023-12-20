package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"
)

const pollDuration = 100 * time.Millisecond

func main() {
	for {
		time.Sleep(pollDuration)
		files, err := os.ReadDir("/control/commands")
		if err != nil {
			continue
		}
		for _, file := range files {
			name := file.Name()
			if name == "done" {
				return
			}
			commandJSON, err := os.ReadFile("/control/commands/" + name)
			if err != nil {
				log.Fatalln("Failed to read command", err)
			}
			var command []string
			if err := json.Unmarshal(commandJSON, &command); err != nil {
				log.Fatalln("Failed to unmarshal command", err)
			}
			if err := os.Remove("/control/commands/" + name); err != nil {
				log.Fatalln("Failed to remove command", err)
			}
			if err := os.Chdir("/control/release/terraform"); err != nil {
				log.Fatalln("Failed to change directory", err)
			}
			cmd := exec.Command(command[0], command[1:]...)

			conn, err := net.Dial("unix", "/control/output/"+name)
			if err != nil {
				log.Fatalln("Failed to dial", err)
			} else {
				log.Printf("Connected to %s\n", "/control/output/"+name)
			}
			defer conn.Close()

			cmd.Stdout = conn
			cmd.Stderr = conn

			status := uint8(0)
			if err := cmd.Start(); err != nil {
				log.Fatalf("Failed to start command: %v\n", err)
			}
			if err := cmd.Wait(); err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					status = uint8(exitError.ExitCode())
				} else {
					log.Fatalf("Failed to wait for command: %v\n", err)
				}
			}
			if err := os.WriteFile("/control/output/"+name+".status", []byte(fmt.Sprintf("%d", status)), 0777); err != nil {
				log.Fatalln("Failed to write status", err)
			}
		}
	}
}
