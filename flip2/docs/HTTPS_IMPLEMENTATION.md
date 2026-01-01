# HTTPS/TLS Implementation Guide for FLIP2

**Date**: 2025-12-16
**Author**: claude-mac
**Status**: ✅ Working on Mac

---

## Overview

This document describes the changes made to enable HTTPS/TLS on the FLIP2 daemon using PocketBase's native TLS support with custom certificates.

---

## Step 1: Generate Self-Signed Certificates

```bash
# Create certs directory
mkdir -p certs

# Generate ECDSA P-384 certificate (valid 1 year)
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-384 \
  -days 365 -nodes \
  -keyout certs/flip2.key \
  -out certs/flip2.crt \
  -subj "/C=US/O=FLIP2/CN=your-hostname" \
  -addext "subjectAltName=DNS:localhost,DNS:your-hostname,IP:127.0.0.1,IP:YOUR_IP_ADDRESS"
```

**For Windows**, replace:
- `CN=your-hostname` → `CN=windows-desktop`
- `IP:YOUR_IP_ADDRESS` → `IP:192.168.1.220`

**For Mac**, we used:
- `CN=mac-laptop`
- `IP:192.168.1.53`

---

## Step 2: Update Config Struct (internal/config/config.go)

Add `TLSConfig` struct and add it to `PocketBaseConfig`:

```go
type PocketBaseConfig struct {
    Host     string    `yaml:"host"`
    Port     int       `yaml:"port"`
    DataDir  string    `yaml:"data_dir"`
    Database string    `yaml:"database"`
    TLS      TLSConfig `yaml:"tls"`  // ADD THIS
}

// ADD THIS STRUCT
type TLSConfig struct {
    Enabled  bool   `yaml:"enabled"`
    CertFile string `yaml:"cert_file"`
    KeyFile  string `yaml:"key_file"`
}
```

---

## Step 3: Update Config YAML (config/config.yaml)

Add TLS section under `pocketbase`:

```yaml
flip2:
  pocketbase:
    host: 0.0.0.0
    port: 8091
    data_dir: ./pb_data
    tls:                          # ADD THIS SECTION
      enabled: true
      cert_file: ./certs/flip2.crt
      key_file: ./certs/flip2.key
```

---

## Step 4: Update Daemon (internal/daemon/daemon.go)

### 4a. Add Import

```go
import (
    "crypto/tls"  // ADD THIS
    // ... other imports
)
```

### 4b. Update Start() - Change os.Args

Replace the os.Args setup with conditional HTTPS/HTTP:

```go
// Start PocketBase (this blocks until it's ready to serve)
listenAddr := fmt.Sprintf("%s:%d", d.config.Flip2.PocketBase.Host, d.config.Flip2.PocketBase.Port)

// Override os.Args to force PocketBase to run 'serve' command
if d.config.Flip2.PocketBase.TLS.Enabled {
    // HTTPS mode with TLS certificates
    os.Args = []string{
        "pocketbase", "serve",
        "--https", listenAddr,
        "--dir", d.config.Flip2.PocketBase.DataDir,
    }
    d.logger.Info("Starting PocketBase HTTPS server",
        "addr", listenAddr,
        "cert", d.config.Flip2.PocketBase.TLS.CertFile,
        "key", d.config.Flip2.PocketBase.TLS.KeyFile)
} else {
    // HTTP mode
    os.Args = []string{
        "pocketbase", "serve",
        "--http", listenAddr,
        "--dir", d.config.Flip2.PocketBase.DataDir,
    }
    d.logger.Info("Starting PocketBase HTTP server", "addr", listenAddr)
}
```

### 4c. Add TLS OnServe Hook in registerHooks()

Add this **BEFORE** other OnServe hooks (like API key middleware):

```go
// TLS Configuration (if enabled)
if d.config.Flip2.PocketBase.TLS.Enabled {
    d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
        cert, err := tls.LoadX509KeyPair(
            d.config.Flip2.PocketBase.TLS.CertFile,
            d.config.Flip2.PocketBase.TLS.KeyFile,
        )
        if err != nil {
            d.logger.Error("Failed to load TLS certificates", "error", err)
            return err
        }

        e.Server.TLSConfig = &tls.Config{
            Certificates: []tls.Certificate{cert},
            MinVersion:   tls.VersionTLS12,
        }
        d.logger.Info("TLS configured",
            "cert", d.config.Flip2.PocketBase.TLS.CertFile,
            "min_version", "TLS1.2")

        return e.Next() // CRITICAL: Always call Next()
    })
}
```

**⚠️ CRITICAL**: The OnServe hook MUST call `e.Next()` at the end. Failing to do so will prevent PocketBase from registering routes, causing all endpoints to return 404.

---

## Step 5: Build and Test

```bash
# Build
go build -o flip2d ./cmd/flip2d

# Restart daemon
pkill -f flip2d
FLIP2_FOREGROUND=1 ./flip2d serve --config config/config.yaml &

# Test HTTPS
curl -sk https://localhost:8091/api/health
# Should return: {"message":"API is healthy.","code":200,"data":{}}

# Verify certificate
echo | openssl s_client -connect localhost:8091 2>/dev/null | openssl x509 -noout -subject -dates
```

---

## Step 6: Update Peer Configuration

For peers connecting to an HTTPS server with self-signed certificates:

1. Update peer URL to use `https://`:
   ```yaml
   peers:
     - id: mac
       url: https://192.168.1.53:8091  # Changed from http://
   ```

2. Ensure HTTPPeer has `InsecureSkipVerify` for self-signed certs (already implemented):
   ```go
   httpClient: &http.Client{
       Transport: &http.Transport{
           TLSClientConfig: &tls.Config{
               InsecureSkipVerify: true,
           },
       },
   }
   ```

---

## Files Changed

| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `TLSConfig` struct, added `TLS` field to `PocketBaseConfig` |
| `config/config.yaml` | Added `tls` section under `pocketbase` |
| `internal/daemon/daemon.go` | Added `crypto/tls` import, conditional `--https` flag, TLS OnServe hook |
| `certs/flip2.crt` | New - TLS certificate |
| `certs/flip2.key` | New - TLS private key |

---

## Verification

### Mac Server (HTTPS Enabled)
```
Server started at https://0.0.0.0:8091
TLS configured cert=./certs/flip2.crt min_version=TLS1.2
```

### Test Results
- `https://localhost:8091/api/health` → 200 OK ✅
- `https://192.168.1.53:8091/api/health` → 200 OK ✅
- Certificate: ECDSA P-384, valid until Dec 2026 ✅
- Routes: All working (no 404 issues) ✅

---

## Common Issues

### Issue: All routes return 404 after enabling TLS

**Cause**: OnServe hook not calling `e.Next()`

**Solution**: Ensure your TLS hook ends with `return e.Next()`

### Issue: Certificate errors when connecting

**Cause**: Self-signed certificate not trusted

**Solution**: Use `-k` flag with curl, or add `InsecureSkipVerify: true` in HTTPPeer

### Issue: TLS handshake fails

**Cause**: Certificate doesn't include correct SAN (Subject Alternative Name)

**Solution**: Regenerate cert with correct IP/DNS in `-addext "subjectAltName=..."`

---

## Security Notes

- Self-signed certificates are suitable for development and trusted local networks
- For production internet deployment, use proper CA-signed certificates (Let's Encrypt recommended)
- Consider using a reverse proxy (nginx/caddy) for production TLS termination
