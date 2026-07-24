# MeowFlixEmby FFI event schema

The C-shared library (`api/ffi`) delivers lifecycle events to a host-registered
callback as NUL-terminated JSON strings.

## Registering

Register the callback **before** calling `MeowflixStart` so you catch the
initial `starting` event:

```c
#include "meowflix.h"

void on_event(const char* json) {
    // json is valid only for the duration of this call — copy if needed.
    printf("event: %s\n", json);
}

int main(void) {
    MeowflixSetEventCallback(on_event);
    MeowflixStart("meowflix.yaml");
    // ... run your event loop ...
    MeowflixStop(10000); // wait up to 10s for a clean stop
    return 0;
}
```

The callback is invoked from a Go goroutine, so the host must be thread-safe.
The pointer passed in is owned by the library and freed immediately after the
call returns — copy the bytes if you need to retain them.

## Event envelope

Every event is a JSON object:

```json
{
  "type": "running",
  "time": "2026-07-24T14:47:03Z",
  "message": "daemon started"
}
```

| Field     | Type   | Notes                                              |
|-----------|--------|----------------------------------------------------|
| `type`    | string | One of the event types below.                      |
| `time`    | string | RFC3339 UTC timestamp.                             |
| `message` | string | Human-readable detail. May be empty.               |

### Event types

| `type`     | When it fires                                            |
|------------|----------------------------------------------------------|
| `starting` | `MeowflixStart` accepted the config and is spinning up.  |
| `running`  | The daemon goroutine has begun `Run`.                    |
| `stopped`  | The daemon exited cleanly (e.g. after `MeowflixStop`).   |
| `error`    | The daemon exited with an error; `message` has details.  |

`message` on an `error` event mirrors `MeowflixLastError()`.

## Exported functions

| Function                              | Returns | Description                                                        |
|---------------------------------------|---------|--------------------------------------------------------------------|
| `MeowflixVersion()`                   | `char*` | Library version. Free with `MeowflixFreeString`.                   |
| `MeowflixSetEventCallback(cb)`        | void    | Register/clear (NULL) the event callback.                          |
| `MeowflixStart(cfgPath)`              | `int`   | 0 ok; 1 already running; 2 config error. Non-blocking.             |
| `MeowflixStop(timeoutMs)`             | `int`   | 0 clean stop; 1 not running; 2 timeout. `<=0` waits indefinitely.  |
| `MeowflixIsRunning()`                 | `int`   | 1 if running, else 0.                                              |
| `MeowflixLastError()`                 | `char*` | Last error message (empty if none). Free with `MeowflixFreeString`.|
| `MeowflixFreeString(s)`               | void    | Free a `char*` returned by this library.                           |

Only one daemon instance runs per process. Call `MeowflixStop` before starting
again.
