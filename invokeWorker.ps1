param ($Worker, $Request)
$pipeOut = New-Object -TypeName System.IO.Pipes.AnonymousPipeServerStream -ArgumentList 'Out','Inheritable'
$pipeIn = New-Object -TypeName System.IO.Pipes.AnonymousPipeServerStream -ArgumentList 'In','Inheritable'
$proc = Start-Process -NoNewWindow -PassThru -FilePath $Worker -ArgumentList spawnclient,$pipeOut.GetClientHandleAsString(),$pipeIn.GetClientHandleAsString()
$content = [System.Text.Encoding]::Unicode.GetBytes([System.IO.File]::ReadAllText($Request))
$pipeOut.Write([BitConverter]::GetBytes(1), 0, 4) # JobRequest
$pipeOut.Write([BitConverter]::GetBytes($content.Length), 0, 4) # Size
$pipeOut.Write($content, 0, $content.Length) # Body
$pipeOut.Flush()
Wait-Process -InputObject $proc
$pipeOut.Dispose()
$pipeIn.Dispose()