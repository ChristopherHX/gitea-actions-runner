# This script can be used to call Runner.Worker as github-act-runner worker on unix like systems
# You just have to create simple .runner file in the root folder with the following Content
# {"isHostedServer": false, "agentName": "my-runner", "workFolder": "_work"}
# Then use `python3 path/to/this/script.py path/to/actions/runner/bin/Runner.Worker` as the worker args

import sys
import subprocess
import os
import threading
import codecs
import json

worker = sys.argv[1]

runner_file = os.path.abspath(os.path.join(worker), '../..') + '/.runner'
if not os.path.exists(runner_file):
    data = {
        'isHostedServer': False,
        'agentName': 'my-runner',
        'workFolder': '_work'
    }
    with open(runner_file, 'w') as file:
        json.dump(data, file)

wdr, wdw = os.pipe()
rdr, rdw = os.pipe()

def readfull(fd: int, l: int):
    b = bytes()
    while len(b) < l:
        r = os.read(fd, l - len(b))
        if len(r) <= 0:
            raise RuntimeError("unexpected read len: {}".format(len(r)))
        b += r
    if len(b) != l:
        raise RuntimeError("read {} bytes expected {} bytes".format(len(b), l))
    return b

def writefull(fd: int, buf: bytes):
    written: int = 0
    while written < len(buf):
        w = os.write(fd, buf[written:])
        if w <= 0:
            raise RuntimeError("unexpected write result: {}".format(w))
        written += w
    if written != len(buf):
        raise RuntimeError("written {} bytes expected {}".format(written, len(buf)))
    return written

def redirectio():
    while(True):
        stdin = sys.stdin.fileno()
        messageType = int.from_bytes(readfull(stdin, 4), "big", signed=False)
        writefull(rdw, messageType.to_bytes(4, sys.byteorder, signed=False))
        messageLength = int.from_bytes(readfull(stdin, 4), "big", signed=False)
        rawmessage = readfull(stdin, messageLength)
        message = codecs.decode(rawmessage, "utf-8")
        if os.getenv("ACTIONS_RUNNER_WORKER_DEBUG", "0") != "0":
            print("Message Received")
            print("Type:", messageType)
            print("================")
            print(message)
            print("================")
        encoded = message.encode("utf_16")[2:]
        writefull(rdw, len(encoded).to_bytes(4, sys.byteorder, signed=False))
        writefull(rdw, encoded)

threading.Thread(target=redirectio, daemon=True).start()

interpreter = []
if worker.endswith(".dll"):
    interpreter = [ "dotnet" ]

code = subprocess.call(interpreter + [worker, "spawnclient", format(rdr), format(wdw)], pass_fds=(rdr, wdw))
print(code)
# https://github.com/actions/runner/blob/af6ed41bcb47019cce2a7035bad76c97ac97b92a/src/Runner.Common/Util/TaskResultUtil.cs#L13-L14
if code >= 100 and code <= 105:
    sys.exit(0)
else:
    sys.exit(1)
