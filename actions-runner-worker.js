const fs = require('fs');
const path = require('path');
const child_process = require('child_process');

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

let inputBuffer = Buffer.alloc(0);

// Listen for incoming data on standard input.
process.stdin.on('data', chunk => {
  inputBuffer = Buffer.concat([inputBuffer, chunk]);
  processMessages();
});

function processMessages() {
  // We need at least 8 bytes to get message type (4 bytes) and length (4 bytes)
  while (inputBuffer.length >= 8) {
    // Read the first 4 bytes as a big‑endian unsigned integer (message type)
    const messageType = inputBuffer.readUInt32BE(0);
    // Next 4 bytes give the message length
    const messageLength = inputBuffer.readUInt32BE(4);
    
    // If we don’t yet have the full payload, wait
    if (inputBuffer.length < 8 + messageLength) break;
    
    const rawMessage = inputBuffer.subarray(8, 8 + messageLength);
    inputBuffer = inputBuffer.subarray(8 + messageLength);
    
    const message = rawMessage.toString('utf8');
    
    // For debugging, if the environment variable is set:
    if (process.env.ACTIONS_RUNNER_WORKER_DEBUG === '1') {
      console.log("Message Received");
      console.log("Type:", messageType);
      console.log("================");
      console.log(message);
      console.log("================");
    }
    
    let encoded = Buffer.from(message, 'utf16le');
    
    const typeBuffer = Buffer.alloc(4);
    typeBuffer.writeUint32LE(messageType, 0);
    const lengthBuffer = Buffer.alloc(4);
    lengthBuffer.writeUint32LE(encoded.length, 0);
    
    childPipeWrite.write(typeBuffer);
    childPipeWrite.write(lengthBuffer);
    childPipeWrite.write(encoded);
  }
}

child.on('exit', (code) => {
  console.log(`Child exited with code ${code}`);
  if (code >= 100 && code <= 105) {
    process.exit(0);
  } else {
    process.exit(1);
  }
});
