property _cli : cs._CLI

Class constructor($target : Text)
	This._cli:=cs._CLI.new("uuid-test"; $target)

Function generate()->$result : Object
	$result:=This._decode(This._cli.execute(New collection("generate")); "uuid")

Function version()->$result : Object
	$result:=This._decode(This._cli.execute(New collection("--version")); "version")

Function _decode($result : Object; $field : Text)->$decoded : Object
	var $payload : Variant
	var $previous : Text
	var $parseErrors : Collection
	var CLI_LastErrors : Collection
	If (Not($result.ok))
		return $result
	End if
	If ((Length($result.stdout)<2) | (Position("\n"; $result.stdout; *)#Length($result.stdout)) | (Position("\r"; $result.stdout; *)>0))
		$result.ok:=False
		$result.error:=New object("code"; "invalid_protocol"; "message"; "CLI response must be exactly one JSON line ending in LF")
		return $result
	End if
	$previous:=Method called on error(ek local)
	CLI_LastErrors:=Null
	ON ERR CALL("_CaptureError"; ek local)
	$payload:=JSON Parse($result.stdout)
	ON ERR CALL($previous; ek local)
	$parseErrors:=CLI_LastErrors
	CLI_LastErrors:=Null
	If (($parseErrors#Null) | (Value type($payload)#Is object))
		$result.ok:=False
		$result.error:=New object("code"; "invalid_protocol"; "message"; "CLI response must be a JSON object"; "details"; $parseErrors)
		return $result
	End if
	If ((Value type($payload.protocol)#Is real) | (Value type($payload[$field])#Is text))
		$result.ok:=False
		$result.error:=New object("code"; "invalid_protocol"; "message"; "CLI response has invalid field types")
		return $result
	End if
	If ($payload.protocol#1)
		$result.ok:=False
		$result.error:=New object("code"; "invalid_protocol"; "message"; "Unsupported CLI protocol version")
		return $result
	End if
	Case of
		: ($field="uuid")
			If ((Length($payload.uuid)#36) | Not(Match regex("^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"; $payload.uuid)))
				$result.ok:=False
				$result.error:=New object("code"; "invalid_protocol"; "message"; "CLI response must contain a canonical lowercase UUIDv7")
				return $result
			End if
		: ($field="version")
			If ($payload.version#"0.1.0")
				$result.ok:=False
				$result.error:=New object("code"; "invalid_protocol"; "message"; "CLI version does not match component version")
				return $result
			End if
		Else
			$result.ok:=False
			$result.error:=New object("code"; "invalid_protocol"; "message"; "Unknown response field")
			return $result
	End case
	$result.value:=$payload
	$decoded:=$result
