//%attributes = {"invisible":true}
var Host_TestStatusFile : 4D.File
var Host_TestReport : Object
Host_TestReport.success:=False
Host_TestReport.unexpectedErrors:=Last errors
Host_TestStatusFile.setText(JSON Stringify(Host_TestReport))
QUIT 4D
