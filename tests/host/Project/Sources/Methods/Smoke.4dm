//%attributes = {"invisible":true}
var $ignored : Integer
var $parameter; $name; $target; $text : Text
var $request; $result; $report : Object
var $tool : cs.UuidTest.UUIDv7
var $requestFile; $statusFile; $probeFile : 4D.File
var $cases : Collection
var Host_TestStatusFile : 4D.File
var Host_TestReport : Object
var $previousHandler : Text
$ignored:=Get database parameter(User param value; $parameter)
If (Is Windows)
	$requestFile:=File($parameter; fk platform path)
Else
	$requestFile:=File($parameter; fk posix path)
End if
$request:=JSON Parse($requestFile.getText())
If (Is Windows)
	$statusFile:=File($request.status; fk platform path)
Else
	$statusFile:=File($request.status; fk posix path)
End if
$statusFile.setText("{\"success\":false,\"phase\":\"starting\"}")
$target:="macOS-universal"
If (Is Windows)
	$target:="Windows-x64"
End if
$tool:=cs.UuidTest.UUIDv7.new($target)
$cases:=New collection(1; 2; 3; 4; 5; 6)
$report:=New object("success"; True; "finished"; False; "cases"; New collection)
Host_TestStatusFile:=$statusFile
Host_TestReport:=$report
$previousHandler:=Method called on error(ek local)
ON ERR CALL("HostFailure"; ek local)
$report.compiled:=$tool._cli.compiled
If ($request.expectCompiled & Not($report.compiled))
	$report.success:=False
End if
For each ($ignored; $cases)
	$result:=$tool.generate()
	$report.cases.push($result)
	If (Not($result.ok))
		$report.success:=False
	Else
		If (Not(Match regex("^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"; $result.value.uuid)))
			$report.success:=False
		End if
	End if
End for each
$result:=$tool._cli.execute(New collection("generate"; "unexpected"))
$report.invalid:=$result
If ($result.ok | ($result.exitCode#2) | ($result.stdout#"") | ($result.error.code#"cli_error"))
	$report.success:=False
End if
If (JSON Parse($result.stderr).error.code#"invalid_arguments")
	$report.success:=False
End if
$result:=$tool.version()
$report.version:=$result
If (Not($result.ok))
	$report.success:=False
Else
	If ($result.value.version#"0.1.0")
		$report.success:=False
	End if
End if
$result:=cs.UuidTest._CLI.new("missing-cli"; $target).execute(New collection("--version"))
$report.missing:=$result
If ($result.error.code#"missing_executable")
	$report.success:=False
End if
$result:=cs.UuidTest.UUIDv7.new("unsupported-arm-target").generate()
$report.unsupported:=$result
If ($result.error.code#"unsupported_architecture")
	$report.success:=False
End if
$result:=cs.UuidTest._Process.new().execute($requestFile; New collection)
$report.launch:=$result
If ($result.error.code#"launch_failed")
	$report.success:=False
End if
$result:=$tool._decode(New object("ok"; True; "stdout"; "not json\n"); "uuid")
$report.protocol:=$result
If ($result.error.code#"invalid_protocol")
	$report.success:=False
End if
$report.invalidPayloads:=New collection
For each ($text; New collection("00000000-0000-4000-8000-000000000000"; "00000000-0000-7000-c000-000000000000"; "00000000-0000-7000-A000-000000000000"; "not a uuid"; "00000000-0000-7000-8000-000000000000\n"; ""))
	$result:=$tool._decode(New object("ok"; True; "stdout"; JSON Stringify(New object("protocol"; 1; "uuid"; $text))+"\n"); "uuid")
	$report.invalidPayloads.push($result)
	If ($result.ok | ($result.error.code#"invalid_protocol"))
		$report.success:=False
	End if
End for each
$result:=$tool._decode(New object("ok"; True; "stdout"; "{\"protocol\":2,\"uuid\":\"00000000-0000-7000-8000-000000000000\"}\n"); "uuid")
$report.protocolVersion:=$result
If ($result.ok | ($result.error.code#"invalid_protocol"))
	$report.success:=False
End if
$result:=$tool._decode(New object("ok"; True; "stdout"; "{\"protocol\":1,\"version\":\"9.0.0\"}\n"); "version")
$report.versionMismatch:=$result
If ($result.ok | ($result.error.code#"invalid_protocol"))
	$report.success:=False
End if
$report.invalidFrames:=New collection
For each ($text; New collection("{\"protocol\":1,\"uuid\":\"00000000-0000-7000-8000-000000000000\"}"; "{}\n{}\n"; "{\"protocol\":true,\"uuid\":\"00000000-0000-7000-8000-000000000000\"}\n"; "{\"protocol\":1}\n"; "[]\n"))
	$report.phase:="decode frame: "+$text
	$statusFile.setText(JSON Stringify($report))
	$result:=$tool._decode(New object("ok"; True; "stdout"; $text); "uuid")
	$report.invalidFrames.push($result)
	If ($result.ok | ($result.error.code#"invalid_protocol"))
		$report.success:=False
	End if
End for each
$report.phase:="argument round trips"
If (Is Windows)
	$probeFile:=File($request.probe; fk platform path)
Else
	$probeFile:=File($request.probe; fk posix path)
End if
$report.roundTrips:=New collection
For each ($name; New collection(""; "path with spaces"; "quote \" and \\ slash"; "trailing\\"; "line\nbreak"; "日本語"; "$() ; & % !"))
	$report.phase:="execute probe: "+$name
	$statusFile.setText(JSON Stringify($report))
	$result:=cs.UuidTest._Process.new().execute($probeFile; New collection($name; $requestFile; $requestFile.parent))
	$report.phase:="decode probe: "+$name
	$report.roundTrips.push($result)
	$statusFile.setText(JSON Stringify($report))
	If (Not($result.ok))
		$report.success:=False
	Else
		$cases:=JSON Parse($result.stdout).arguments
		If (($cases.length#3) | ($cases[0]#$name) | ($cases[1]#$parameter) | ($result.stderr#"probe diagnostic\n") | ($result.exitCode#0))
			$report.success:=False
		End if
		If (Is Windows)
			$text:=$requestFile.parent.platformPath
		Else
			$text:=$requestFile.parent.path
		End if
		If ($cases[2]#$text)
			$report.success:=False
		End if
	End if
End for each
$report.phase:="NUL argument"
$statusFile.setText(JSON Stringify($report))
$result:=cs.UuidTest._Process.new().execute($probeFile; New collection(Char(0)))
$report.nul:=$result
If ($result.ok | ($result.error.code#"invalid_argument"))
	$report.success:=False
End if
$report.phase:="numeric argument"
$statusFile.setText(JSON Stringify($report))
$result:=cs.UuidTest._Process.new().execute($probeFile; New collection(123))
$report.argumentType:=$result
If ($result.ok | ($result.error.code#"invalid_argument"))
	$report.success:=False
End if
$report.finished:=True
$report.phase:="complete"
$statusFile.setText(JSON Stringify($report))
ON ERR CALL($previousHandler; ek local)
QUIT 4D
