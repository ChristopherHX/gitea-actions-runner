param ($Worker)
# This script can be used to call Runner.Worker as github-act-runner worker
# You just have to create simple .runner file in the root folder with the following Content
# {"isHostedServer": false, "agentName": "my-runner", "workFolder": "_work"}
# Then use `pwsh path/to/this/script.ps1 path/to/actions/runner/bin/Runner.Worker` as the worker args

# Fallback if not existing
$runnerFile = (Join-Path $Worker "../.." -Resolve)
if(-not (Test-Path $runnerFile))  {                                                                              
    Write-Output '{"isHostedServer": false, "agentName": "my-runner", "workFolder": "_work"}' | Out-File $runnerFile
}

$stdin = [System.Console]::OpenStandardInput()
$pipeOut = New-Object -TypeName System.IO.Pipes.AnonymousPipeServerStream -ArgumentList 'Out','Inheritable'
$pipeIn = New-Object -TypeName System.IO.Pipes.AnonymousPipeServerStream -ArgumentList 'In','Inheritable'
if($Worker.EndsWith(".dll")) {
    $proc = Start-Process -NoNewWindow -PassThru -FilePath dotnet -ArgumentList $Worker,spawnclient,$pipeOut.GetClientHandleAsString(),$pipeIn.GetClientHandleAsString()
} else {
    $proc = Start-Process -NoNewWindow -PassThru -FilePath $Worker -ArgumentList spawnclient,$pipeOut.GetClientHandleAsString(),$pipeIn.GetClientHandleAsString()
}
$inputjob = Start-ThreadJob -ScriptBlock {
    $stdin = $using:stdin
    $pipeOut = $using:pipeOut
    $pipeIn = $using:pipeIn
    $proc = $using:proc
    $buf = New-Object byte[] 4
    function ReadStdin($buf, $offset, $len) {
        # We should read exactly, if available
        if($stdin.ReadExactly) {
            $stdin.ReadExactly($buf, $offset, $len)
        } else {
            # broken fallback
            $stdin.Read($buf, $offset, $len)
        }
    }
    while( -Not $proc.HasExited ) {
        ReadStdin $buf 0 4
        $messageType = [System.Buffers.Binary.BinaryPrimitives]::ReadInt32BigEndian($buf)
        if($proc.HasExited) {
            return
        }
        if($messageType -eq 0) {
            return
        }
        ReadStdin $buf 0 4
        $contentLength = [System.Buffers.Binary.BinaryPrimitives]::ReadInt32BigEndian($buf)
        $rawcontent = New-Object byte[] $contentLength
        ReadStdin $rawcontent 0 $contentLength
        $utf8Content = [System.Text.Encoding]::UTF8.GetString($rawcontent)
        $content = [System.Text.Encoding]::Unicode.GetBytes($utf8Content)
        $pipeOut.Write([BitConverter]::GetBytes($messageType), 0, 4)
        $pipeOut.Write([BitConverter]::GetBytes($content.Length), 0, 4)
        $pipeOut.Write($content, 0, $content.Length)
        $pipeOut.Flush()
    }
}
echo "Wait for exit"
Wait-Process -InputObject $proc
$exitCode = $proc.ExitCode
# https://github.com/actions/runner/blob/af6ed41bcb47019cce2a7035bad76c97ac97b92a/src/Runner.Common/Util/TaskResultUtil.cs#L13-L14
if(($exitCode -ge 100) -and ($exitCode -le 105)) {
    $conclusion = 0
} else {
    $conclusion = 1
}
echo "Has exited with code $exitCode and conclusion $conclusion"
# This is needed to shutdown the input thread, it seem to stall if we just do nothing or exit
[System.Environment]::Exit($conclusion)