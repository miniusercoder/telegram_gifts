# C-shared Go library for Telegram Stars

This project builds a C-shared library **`libtg.so`** with generated **`libtg.h`**.
It wraps a Telegram client via [`gogram`](https://github.com/amarnathcjd/gogram) and exposes a small C ABI for FFI users.

The library can:

- send **Star Gifts** by username
- send a regular Telegram message by username
- read your **Stars balance**
- get your own Telegram username
- validate and cache a recipient username

A single global Telegram client is used under the hood. Call `Init` once, use the API, then call `Close` when the process no longer needs the client.

---

## Features

- C-friendly API: `Init`, `GetStarsBalance`, `GetMyUsername`, `FreeCString`, `ValidateRecipient`, `SendGift`, `SendMessage`, `Close`
- Long-lived global Telegram client between `Init` and `Close`
- Session-file advisory lock via `<sessionFile>.lock`
- Basic guard against corrupted/truncated session files
- Username normalization: trims whitespace, removes leading `@`, lowercases
- Recipient resolve cache for `ValidateRecipient` -> `SendGift`
- No updates polling, cache disabled
- Optional proxy argument for `Init`
- Builds via Docker + Makefile into `./dist`

---

## Build

### Recommended: Docker + Makefile

Prerequisites:

- Docker with BuildKit
- `make`

Build:

```bash
make build-libtg
```

Result:

- `dist/libtg.so` - shared library
- `dist/libtg.h` - generated C header

### Manual build

Prerequisites:

- Go
- `gcc` or `clang`
- `CGO_ENABLED=1`

Example for Linux/amd64:

```bash
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

go build \
  -buildmode=c-shared \
  -o libtg.so \
  -trimpath \
  -buildvcs=false \
  -ldflags "-s -w -buildid="
```

This produces `libtg.so` and `libtg.h` in the current directory.

---

## C API

Include the generated header:

```c
#include "libtg.h"
```

Current exported functions:

```c
// Initializes the global Telegram client.
// Must be called before other functions.
// proxy may be NULL or an empty string.
// Returns 0 on success, negative code on error.
int Init(int appId, char* appHash, char* sessionFile, char* proxy);

// Returns Stars balance (>= 0) or a negative error code.
long long GetStarsBalance(void);

// Returns your username without '@' as a newly allocated C string.
// Empty string on error. Free with FreeCString().
char* GetMyUsername(void);

// Frees strings allocated by this library, for example GetMyUsername().
void FreeCString(char* ptr);

// Validates that username resolves to a Telegram user.
// Also caches user id/access hash for SendGift().
// Returns 0 on success, negative code on failure.
int ValidateRecipient(char* username);

// Sends a Star Gift.
// username - recipient username, with or without leading '@'
// giftID   - Star Gift ID
// hideName - 0 shows sender, non-zero hides sender
// comment  - gift message text
// Returns 0 on success, negative code on failure.
int SendGift(char* username, long long giftID, int hideName, char* comment);

// Sends a regular Telegram message.
// Returns 0 on success, negative code on failure.
int SendMessage(char* username, char* message);

// Stops the global client, clears the recipient cache, and releases the session lock.
// Safe to call even if the client is not initialized.
int Close(void);
```

---

## Error Codes

Used by `Init`, `ValidateRecipient`, `SendGift`, `SendMessage`, and as negative return values of `GetStarsBalance`.

| Code | Name                    | Meaning                                      |
| ---: | ----------------------- | -------------------------------------------- |
|    0 | `Success`               | Success                                      |
|   -1 | `ErrCreateClient`       | Failed to create client or proxy             |
|   -2 | `ErrStartClient`        | Failed to start/login client                 |
|   -3 | `ErrResolveUsername`    | Username did not resolve to a user           |
|   -4 | `ErrGetPaymentForm`     | Stars status/payment form API failed         |
|   -5 | `ErrSendStars`          | Sending gift/message failed                  |
|   -6 | `ErrClientNotInitialized` | API called before successful `Init`        |
|   -7 | `ErrSessionLocked`      | Session file is locked by another process    |
|   -8 | `ErrBadSessionFile`     | Session file looks corrupted/truncated       |

---

## Usage Overview

1. Create a Telegram API app at [https://my.telegram.org](https://my.telegram.org) and get `appId` and `appHash`.

2. Choose a session file path, for example `./stars.dat`. On first `Init`, gogram may ask for phone/code/2FA interactively if the session is not logged in yet.

3. Initialize the client:

   ```c
   int rc = Init(APP_ID, "APP_HASH", "./stars.dat", "");
   if (rc != 0) {
       // handle error
   }
   ```

4. Optionally pass a proxy URL:

   ```c
   int rc = Init(APP_ID, "APP_HASH", "./stars.dat", "socks5://user:pass@127.0.0.1:1080");
   ```

5. Use the API:

   ```c
   long long balance = GetStarsBalance();

   char* me = GetMyUsername();
   // use me
   FreeCString(me);

   if (ValidateRecipient("durov") == 0) {
       int gift_rc = SendGift("durov", 987654321LL, 1, "hello");
       int msg_rc = SendMessage("durov", "hello");
       (void)gift_rc;
       (void)msg_rc;
   }
   ```

6. Release resources:

   ```c
   Close();
   ```

The client is not started and stopped for every operation anymore. It stays active after `Init`, and operations are serialized inside the Go library with a mutex.

---

## Proxy

`Init` accepts a proxy string. Supported formats are handled by `gogram`'s `ProxyFromURL`, for example:

```text
socks5://user:pass@ip:port
http://user:pass@ip:port
```

If `proxy` is empty, the code falls back to `HardProxyURL` from `main.go`. In the current source it is empty, so no proxy is used unless the caller passes one.

---

## Example: Python `ctypes`

```python
import ctypes
from enum import IntEnum
from threading import RLock


class ErrorCodes(IntEnum):
    SUCCESS = 0
    CREATE_CLIENT_ERROR = -1
    START_CLIENT_ERROR = -2
    INVALID_USERNAME = -3
    PAYMENT_FORM_ERROR = -4
    SEND_ERROR = -5
    CLIENT_NOT_INITIALIZED = -6
    SESSION_LOCKED = -7
    BAD_SESSION_FILE = -8


client_lock = RLock()


class GiftSender:
    def __init__(
        self,
        api_id: int,
        api_hash: str,
        data_file: str = "stars.dat",
        proxy: str = "",
        lib_name: str = "./libtg.so",
    ):
        self.data_file = data_file
        self.lib = ctypes.CDLL(lib_name)
        self.initialized = False

        self.lib.Init.argtypes = [
            ctypes.c_int,
            ctypes.c_char_p,
            ctypes.c_char_p,
            ctypes.c_char_p,
        ]
        self.lib.Init.restype = ctypes.c_int

        self.lib.GetStarsBalance.argtypes = []
        self.lib.GetStarsBalance.restype = ctypes.c_longlong

        # Use c_void_p so Python does not copy the string and lose the original pointer.
        self.lib.GetMyUsername.argtypes = []
        self.lib.GetMyUsername.restype = ctypes.c_void_p

        self.lib.FreeCString.argtypes = [ctypes.c_void_p]
        self.lib.FreeCString.restype = None

        self.lib.ValidateRecipient.argtypes = [ctypes.c_char_p]
        self.lib.ValidateRecipient.restype = ctypes.c_int

        self.lib.SendGift.argtypes = [
            ctypes.c_char_p,
            ctypes.c_longlong,
            ctypes.c_int,
            ctypes.c_char_p,
        ]
        self.lib.SendGift.restype = ctypes.c_int

        self.lib.SendMessage.argtypes = [ctypes.c_char_p, ctypes.c_char_p]
        self.lib.SendMessage.restype = ctypes.c_int

        self.lib.Close.argtypes = []
        self.lib.Close.restype = ctypes.c_int

        with client_lock:
            rc = self.lib.Init(
                api_id,
                api_hash.encode("utf-8"),
                data_file.encode("utf-8"),
                proxy.encode("utf-8"),
            )

        if rc != ErrorCodes.SUCCESS:
            raise RuntimeError(f"Init failed: {ErrorCodes(rc).name}")

        self.initialized = True

    def get_stars_balance(self) -> int:
        with client_lock:
            return self.lib.GetStarsBalance()

    def get_my_username(self) -> str:
        with client_lock:
            ptr = self.lib.GetMyUsername()
        if not ptr:
            return ""
        try:
            return ctypes.string_at(ptr).decode("utf-8")
        finally:
            self.lib.FreeCString(ptr)

    def validate_recipient(self, username: str) -> bool:
        with client_lock:
            rc = self.lib.ValidateRecipient(username.encode("utf-8"))
        return rc == ErrorCodes.SUCCESS

    def send_gift(self, username: str, gift_id: int, anonymous: bool, comment: str = "") -> int:
        with client_lock:
            return self.lib.SendGift(
                username.encode("utf-8"),
                int(gift_id),
                int(anonymous),
                comment.encode("utf-8"),
            )

    def send_message(self, username: str, message: str) -> int:
        with client_lock:
            return self.lib.SendMessage(
                username.encode("utf-8"),
                message.encode("utf-8"),
            )

    def close(self) -> int:
        with client_lock:
            rc = self.lib.Close()
        self.initialized = False
        return rc
```

---

## Notes

- **Gift IDs**: you must provide a valid Star Gift ID. Listing available gifts is out of scope.
- **Session lock**: `Init` creates and locks `<sessionFile>.lock` until `Close`.
- **Session file guard**: non-empty session files whose size is not a multiple of 16 are rejected before gogram attempts AES decrypt.
- **Thread safety**: Go-side operations are serialized with a mutex. External code may still use its own lock around FFI calls for predictable lifecycle handling.
- **Secrets**: do not commit `appId`, `appHash`, session files, or real proxy credentials.
