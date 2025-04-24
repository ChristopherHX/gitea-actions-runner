// 

const fs = require('fs');
const path = require('path');
const child_process = require('child_process');
const { Duplex } = require('stream');
const http2 = require('http2');
const http = require('http');

// Custom duplex stream that uses process.stdin for reading and process.stdout for writing.
class StdioDuplex extends Duplex {
  constructor(options) {
    super(options);
    // Use process.stdin as the source of incoming data.
    this.stdin = process.stdin;
    this.stdout = process.stdout;

    // As data comes in on stdin, push it to the readable side.
    this.stdin.on('data', (chunk) => {
      // If push returns false, the internal buffer is full; pause stdin.
      if (!this.push(chunk)) {
        this.stdin.pause();
      }
    });
    this.stdin.on('end', () => this.push(null));
  }
  
  // Called when the consumer is ready for more data.
  _read(size) {
    this.stdin.resume();
  }

  // Called when data is written to the stream; forward it to stdout.
  _write(chunk, encoding, callback) {
    this.stdout.write(chunk, encoding, callback);
  }

  // Called when no more data will be written.
  _final(callback) {
    this.stdout.end();
    callback();
  }
}

// Create an instance of the custom duplex stream.
const stdioDuplex = new StdioDuplex();

// Use Node's native HTTP/2 client with a custom connection.
// The "authority" URL here is a placeholder. HTTP/2 requires a proper protocol negotiation,
// so the underlying framing might need additional handling if you're bridging to a non-standard transport.
const client = http2.connect('http://localhost', {
  createConnection: () => stdioDuplex,
  // Additional options might be required depending on your environment.
  sessionTimeout: 60 * 60 * 24 * 7,
});

const server = http.createServer((req, res) => {
    const creq = client.request({
            ':method': req.method,
            ':path': req.url,
        }
    );

    req.pipe(creq)
    
    creq.on('response', (headers) => {
        console.error('Response headers:', headers);
        const status = headers[':status'] || 200;
        // Remove HTTP/2 pseudo headers before forwarding.
        const resHeaders = {};
        for (const name in headers) {
          if (!name.startsWith(':')) {
            resHeaders[name] = headers[name];
          }
        }
        res.writeHead(status, resHeaders);
        creq.pipe(res);
    });

    creq.on('error', (err) => {
        console.error('Error with HTTP/2 request:', err);
        res.writeHead(500);
        res.end('Internal Server Error');
    });
        
    creq.on('end', () => {
        console.error('Response ended.');
    });
});

const hostname = process.argv.length > 3 ? process.argv[3] : "localhost";

server.listen(0, hostname, () => {
  const port = server.address().port;
  console.error(`Server running at http://${hostname}:${port}/`);

  // Get the worker path from the command line.
  const worker = process.argv[2];

  // Compute the runner file path (like os.path.abspath(os.path.join(worker, '../../.runner')))
  const runnerFile = path.resolve(worker, '../../.runner');
  if (!fs.existsSync(runnerFile)) {
    // Create default JSON data
    const data = {
      isHostedServer: false,
      agentName: 'my-runner',
      workFolder: '_work'
    };
    fs.writeFileSync(runnerFile, JSON.stringify(data));
  }

  const interpreter = worker.endsWith('.dll') ? ['dotnet'] : [];

  var spawnArgs = interpreter.concat([worker, "spawnclient", "3", "4"]);
  const exe = spawnArgs.shift();

  const child = child_process.spawn(
    exe, spawnArgs,
    {
      stdio: [process.stdin, process.stdout, process.stderr, 'pipe', 'pipe']
    }
  );

  const childPipeWrite = child.stdio[3];
  const childPipeRead = child.stdio[4];

  const creq = client.request({
      ':method': "GET",
      ':path': "/JobRequest?SYSTEMVSSCONNECTION=" + encodeURIComponent(`http://${hostname}:${port}/`),
  }
  );
  var fchunk = Buffer.alloc(0);
  creq.on('data', (chunk) => {
    console.error('Response data:', chunk.toString());
      fchunk = Buffer.concat([fchunk, chunk]);
  });
  creq.on('end', () => {
    console.error('Response ended.');
    let jobMessage = fchunk.toString();
    console.error('fchunk:', jobMessage);
    let encoded = Buffer.from(jobMessage.toString(), 'utf16le');
    
    // Prepare buffers for the type and encoded-length as 4-byte big-endian integers.
    const typeBuffer = Buffer.alloc(4);
    typeBuffer.writeUint32LE(1, 0);
    const lengthBuffer = Buffer.alloc(4);
    lengthBuffer.writeUint32LE(encoded.length, 0);

    childPipeWrite.write(typeBuffer);
    childPipeWrite.write(lengthBuffer);
    childPipeWrite.write(encoded);

    (() => {
      const creq = client.request({
        ':method': "GET",
        ':path': "/WaitForCancellation"
      }
      );
      var fchunk = Buffer.alloc(0);
      creq.on('data', (chunk) => {
        console.error('Response data:', chunk.toString());
          fchunk = Buffer.concat([fchunk, chunk]);
      });
      creq.on('end', () => {
        console.error('Response ended.');
          let jobMessage = fchunk.toString();
          console.error('fchunk:', jobMessage);
          let encoded = Buffer.from("", 'utf16le');
          
          if (jobMessage.includes("cancelled")) {
          console.error('Cancelled');

          // Prepare buffers for the type and encoded-length as 4-byte big-endian integers.
          const typeBuffer = Buffer.alloc(4);
          typeBuffer.writeUint32LE(2, 0);
          const lengthBuffer = Buffer.alloc(4);
          lengthBuffer.writeUint32LE(encoded.length, 0);
          
          childPipeWrite.write(typeBuffer);
          childPipeWrite.write(lengthBuffer);
          childPipeWrite.write(encoded);

        } else {
          console.error('Not Cancelled');
        }
      });    
    })()
  });

  child.on('exit', (code) => {
    console.error(`Child exited with code ${code}`);
    if (code >= 100 && code <= 105) {
      process.exit(0);
    } else {
      process.exit(1);
    }
  });
});
