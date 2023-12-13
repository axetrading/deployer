import http from 'http'

if (process.argv.length != 4) {
    console.error('Usage: node test-log-receiver.mjs <ip> <port>')
    process.exit(1)
}
const ip = process.argv[2]
const port = parseInt(process.argv[3], 10)

const sessions = {}

const urlBase = `http://${ip}:${port}`

const handlePostSessions = (request, response) => {
    const id = crypto.randomUUID()
    sessions[id] = {
        id,
        nextSequence: 0
    }
    const location = `${urlBase}/sessions/${id}/0`
    response.writeHead(201, { 'Content-Type': 'text/plain', 'Location': location })
    response.end(location)
    console.log(`created session ${id}`)
}

const handlePostSession = (request, response) => {
    const parts = request.url.split('/')
    const id = parts[2]
    const sequence = parseInt(parts[3], 10)
    const session = sessions[id]
    if (! session) {
        response.writeHead(404, { 'Content-Type': 'text/plain' })
        response.end('Not found\n')
        return
    }
    if (sequence != session.nextSequence) {
        response.writeHead(400, { 'Content-Type': 'text/plain' })
        response.end('Bad sequence\n')
        return
    }
    const buffers = []
    request.on('data', (chunk) => {
        buffers.push(chunk)
    })
    request.on('end', () => {
        const body = Buffer.concat(buffers)
        let data
        try {
            data = JSON.parse(body)
        } catch (e) {
            console.error("Bad JSON, sending 400")
            response.writeHead(400, { 'Content-Type': 'text/plain' })
            response.end('Bad JSON\n')
            return
        }
        if (data.done) {
            if (data.error) {
                console.log(`error: ${data.error}`)
            } else {
                console.log(`done.`)
            }
            delete sessions[id]
            response.writeHead(200, { 'Content-Type': 'text/plan' })
            response.end('Done.\n')
            return
        }
        session.nextSequence = sequence + 1
        for (const line of data.lines) {
            console.log("line: " + line)
        }
        response.writeHead(200, { 'Content-Type': 'text/json' })
        response.end(JSON.stringify({ continue: `${urlBase}/sessions/${id}/${session.nextSequence}` }))
    })
}

const server = http.createServer((request, response) => {
    if (request.method == 'POST' && request.url == '/sessions') {
        handlePostSessions(request, response)
    } else if (request.method == 'POST' && request.url.startsWith('/sessions/')) {
        handlePostSession(request, response)
    } else {
        response.writeHead(404, { 'Content-Type': 'text/plain' })
        response.end('Not found\n')
    }
})

server.listen(port, ip, () => {
    console.log(`Create a session with\n\n  curl -XPOST ${urlBase}/sessions\n`)
});

