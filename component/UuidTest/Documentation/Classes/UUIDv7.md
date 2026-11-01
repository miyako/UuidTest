# UUIDv7

`cs.UuidTest.UUIDv7.new({target : Text})` creates the UUIDv7 API.
Inside the component, use `cs.UUIDv7`. Repository, component and namespace
are all exactly `UuidTest`.

On macOS, omit `target` or pass `"macOS-universal"`. On Windows, explicitly
pass `"Windows-x64"`. Other targets return `unsupported_architecture`.
Native Windows ARM64 is not supported.

| Method | Result |
| --- | --- |
| `generate() : Object` | A newly generated UUIDv7 in `result.value.uuid` |
| `version() : Object` | CLI version in `result.value.version` |

Calls are synchronous, close stdin and enforce a 30-second process timeout.
Success is `{ok:true,stdout:Text,stderr:Text,exitCode:0,value:Object}`.
Generation returns `value:{protocol:1,uuid:Text}`; version returns
`value:{protocol:1,version:"0.1.0"}`. The decoder requires one LF-terminated
JSON line, numeric protocol 1,
canonical lowercase UUIDv7 with version 7 and RFC variant bits, and an exact
component/CLI version match. UUIDs use the RFC 9562 section 5.7 layout:
48-bit Unix milliseconds and 74 cryptographically random bits.

Every call starts a fresh CLI process. UUIDs are **not monotonic** within a
millisecond or across clock rollback. This API does not coordinate clocks,
guarantee uniqueness, or provide a sequential identifier service.

Failures are `{ok:false,error:{code,message{,details}}}`. stdout/stderr/exitCode
are retained when available; prelaunch errors do not invent an exit code.
Codes include `missing_executable`, `unsupported_platform`,
`unsupported_architecture`, `invalid_argument`, `launch_failed`,
`process_error`, `process_terminated`, `cli_error`, `invalid_protocol`.
A CLI generation failure exits 1 and writes protocol JSON to stderr with
`clock_error` or `entropy_error`; the adapter returns `cli_error` with that
diagnostic intact in `stderr` and `error.message`. Nonempty stderr alone is
not failure. Output write failures exit 1, with `output_error` JSON if stderr
is writable; stdout may be partial. No empty/Null success fallback, PATH lookup
or runtime download.

```4d
var $tool : cs.UuidTest.UUIDv7
var $result : Object
var $target : Text
$target:="macOS-universal"
If (Is Windows)
  $target:="Windows-x64"
End if
$tool:=cs.UuidTest.UUIDv7.new($target)
$result:=$tool.generate()
If ($result.ok)
  ALERT($result.value.uuid)
Else
  ALERT($result.error.code+": "+$result.error.message)
End if
```

`_cli` and `_decode` are implementation details, accessible only by convention,
not a 4D access-control boundary.
