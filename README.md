# UuidTest

A real **4D component generating UUIDv7** using a bundled, separate Go
executable. Repository, component, project, catalog, namespace and release
basename are all exactly `UuidTest`. Initialized from
[miyako/4d-component-template](https://github.com/miyako/4d-component-template)
using GitHub's template creation flow. Only Go source is retained.

## Host API

Install `UuidTest.4dbase` in your host's `Components/` folder and reload:

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

`generate()` returns `{ok:true,stdout,stderr,exitCode:0,value:{protocol:1,uuid}}`.
`version()` returns the same result envelope with
`value:{protocol:1,version:"0.1.0"}`.
Calls are synchronous with a 30-second timeout.
Failures return `{ok:false,error:{code,message{,details}}}` and preserve
available process output/status. Prelaunch errors do not invent exit codes.

Class contracts: [UUIDv7](component/UuidTest/Documentation/Classes/UUIDv7.md),
[_CLI](component/UuidTest/Documentation/Classes/_CLI.md),
[_Process](component/UuidTest/Documentation/Classes/_Process.md).
The generic resource/execution layer is separate from the feature.
Underscore classes are internal by convention, not access control.

## UUID semantics and CLI protocol

The stdlib-only implementation follows
[RFC 9562 section 5.7](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.7):
48-bit Unix milliseconds, version nibble `7`, variant bits `10`, and 74
random bits from Go `crypto/rand`. Output is lowercase canonical
`8-4-4-4-12` hex. Invalid clock range and entropy read failures are explicit,
with no fallback randomness. No third-party UUID library is needed for this
small, directly tested algorithm.

**Not monotonic:** each call starts a fresh process. UUIDs generated in the
same millisecond are randomly ordered; clock rollback is not corrected.
There is no cross-process counter, stable state, or uniqueness guarantee.
The collision sample in tests is empirical evidence, not a proof.

UTF-8 protocol version 1: exactly one JSON line with LF, on both OSes.

| Invocation | Exit | stdout | stderr |
| --- | --- | --- | --- |
| `uuid-test generate` | 0 | `{"protocol":1,"uuid":"<UUIDv7>"}` | empty |
| `uuid-test --version` | 0 | `{"protocol":1,"version":"0.1.0"}` | empty |
| `uuid-test --help` | 0 | `{"protocol":1,"usage":"uuid-test generate \| --version \| --help"}` | empty |
| Missing/unknown/extra arguments | 2 | empty | protocol error JSON |
| Clock/entropy failure | 1 | empty | protocol error JSON with diagnostic |
| Output write failure | 1 | may be partial | `output_error` JSON if stderr remains writable |

Argument errors are
`{"protocol":1,"error":{"code":"invalid_arguments","message":"expected generate, --version or --help"}}`.
Generation errors use `clock_error` or `entropy_error`. In 4D these become
`cli_error`, preserving the complete diagnostic in `stderr` and `error.message`.
Failed or short output writes never return success; if stderr itself fails,
the CLI exits 1 without attempting recursive diagnostics.
Nonempty stderr alone does not imply failure. No PATH fallback, shell, runtime
download, permission repair, or permanent host-global error handler is used.

## Build and local acceptance

Requirements: Python 3.10+, Go 1.26.10 (CI pin; module minimum 1.23),
macOS CLI 12+ or Windows x64. CGO is disabled.
4D project compatibilityVersion is 2060; local acceptance uses free official
tool4d 20.7 build 101905 on Apple Silicon. Native Windows ARM64 is unsupported.

```sh
python3 -m unittest discover -s tests -v
python3 scripts/generate_icons.py --check
python3 scripts/build_cli.py --target macos-arm64
```

Native CI builds/tests `macos-x64`, `macos-arm64`, and `windows-x64` separately
from the same commit, then `scripts/ci_config.py assemble` combines the macOS
slices with `lipo`. For local Apple Silicon acceptance, Go can also cross-build
Intel macOS and Windows binaries (this does **not** prove native execution):

```sh
mkdir -p build/macos-x64 build/windows-x64
(cd cli && CGO_ENABLED=0 GOTOOLCHAIN=local GOOS=darwin GOARCH=amd64 \
  go build -trimpath -o ../build/macos-x64/uuid-test .)
(cd cli && CGO_ENABLED=0 GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 \
  go build -trimpath -o ../build/windows-x64/uuid-test.exe .)
python3 scripts/ci_config.py assemble
```

Download the official vendor tool into ignored `.tools/`, never Resources:

```sh
export TOOL4D_URL=https://resources-download.4d.com/release/20.x/20.7/101905/mac/tool4d_arm64.tar.xz
export TOOL4D_SHA256=40a5d177a4f73165f52abf6ec8000249d81411c41353ccf258cece8e74639261
python3 scripts/ci_config.py download-tool4d
python3 scripts/test_4d.py --tool4d .tools/tool4d.app/Contents/MacOS/tool4d
python3 scripts/compile_component.py --tool4d .tools/tool4d.app/Contents/MacOS/tool4d
python3 scripts/package_component.py --repository miyako/UuidTest \
  --tag v0.1.0 --unpack-test
python3 scripts/test_4d.py --tool4d .tools/tool4d.app/Contents/MacOS/tool4d \
  --component build/release-test/UuidTest.4dbase
```

The `--tag` argument validates package version identity; it does not create or
publish a tag. Local unsigned compilation/testing needs **no signing secrets,
4D license activation, or repository configuration changes**.

The host fixture loads an isolated source or unpacked compiled component,
calls `cs.UuidTest.UUIDv7`, checks six real generations, version, missing
resources, unsupported targets, launch errors, malformed protocols, and real
argument round trips through `_Process`. A separate Go test probe exercises
Unicode, spaces, quotes, backslashes, empty/newline/metacharacter strings and
File/Folder arguments, with zero exit/nonempty stderr. It is not distributed.
Python additionally validates UUID bytes, timestamp bounds and distinctness.
Go tests inject clock/entropy for deterministic vectors, range/read failures,
all random field masks, and a measured 10,000-UUID collision sample.

Compilation uses actual `Compile project` with `x86_64_generic` and
`arm64_macOS_lib`, requires fresh successful output and binds component inputs
by SHA256. Compiler-generated completion metadata must name `UUIDv7`.
Packaging preserves explicit nested ZIP directory entries needed by the real
4D loader, includes native Libraries externally, and strips all `.4dm` source:

```text
UuidTest.zip
  UuidTest.4dbase/
    UuidTest.4DZ
    Libraries/lib4d-arm64.dylib
    Resources/bin/macOS/uuid-test          universal x86_64 + arm64
    Resources/bin/Windows/uuid-test.exe    x64
    Resources/licenses/...
    Resources/logo.png + logo.svg + en.lproj/...
    Documentation/Classes/...
```

The final host test must report `compiled:true`, evaluated by `Is compiled mode`
inside the component. CLI/Python passes or a compiler success alone are not
acceptance of a usable component. `build/host-status.json` contains the latest
real host report; `dist/UuidTest.zip` is the installable local artifact.

## Releases and limitations

No tag/release is needed for local installation. For a future approved GitHub
release, set repository Actions **variables** `FOURD_ENABLED=true`,
`TOOL4D_URL`, and `TOOL4D_SHA256`, then push matching `v0.1.0`.
The release gate refuses uninitialized/misnamed/missing/stale/failed artifacts.
It compiles, unpacks and loads the final component before publication.
`GITHUB_TOKEN` provides only the publish job's write permission.
The probe build also needs the pinned Go setup in 4D validation jobs.

After a release, add GitHub dependency `miyako/UuidTest` in the host Dependency
manager (20 R6+ / compatible current LTS; confirm your host), or declare:

```json
{"version":2100,"dependencies":{"UuidTest":{"github":"miyako/UuidTest","tag":"v0.1.0"}}}
```

Apple Silicon local acceptance does not prove Windows 4D runtime/quoting,
native Intel 4D loading, dependency-manager download, or signing/notarization.
Native hosted CLI CI is separate evidence. Unsigned downloads may be subject
to Gatekeeper/quarantine; no automatic trust bypass is provided.
No signing/account settings are configured here.

## Licensing and icons

Component, UUID algorithm, tests and original scaffold artwork are MIT:
[LICENSE](LICENSE). Go's statically linked runtime/stdlib are BSD-3-Clause;
their notice is redistributed in `Resources/licenses/Go-LICENSE`.
Vendor tool4d is separately licensed and never bundled.
No third-party UUID algorithm or library source was copied.

Edit `assets/icon-source.svg` only within the supported rectangle/polygon
renderer, then run `python3 scripts/generate_icons.py`. Both icons regenerate:
`logo.png` is real RGBA PNG and `logo.svg` embeds its byte-identical PNG data URI.
See [AGENTS.md](AGENTS.md) for inherited component/build/signing requirements.
