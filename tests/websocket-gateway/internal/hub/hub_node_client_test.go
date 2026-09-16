package hub_test

import (
	"bufio"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"metarang/websocket-gateway/internal/hub"
)

func TestOfficialSocketIOClientConnectAndBroadcast(t *testing.T) {
	node := nodeExecutable(t)
	clientDir := frontendDir(t)
	if _, err := os.Stat(filepath.Join(clientDir, "node_modules", "socket.io-client")); err != nil {
		t.Skip("socket.io-client is not installed in the frontend project")
	}

	h := hub.New(stubValidator{userID: 42}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	script := `
import { io } from "socket.io-client";

const url = process.env.WS_URL;
const socket = io(url, {
  path: "/socket.io/",
  transports: ["websocket", "polling"],
  auth: { token: "valid-token" },
  query: { token: "valid-token" },
  reconnection: false,
  timeout: 8000,
  forceNew: true,
});

const timer = setTimeout(() => {
  console.error("timeout waiting for events");
  socket.close();
  process.exit(1);
}, 12000);

socket.on("connect_error", (err) => {
  console.error("connect_error", err.message || err);
  clearTimeout(timer);
  process.exit(1);
});

socket.on("connected", (data) => {
  if (!data || Number(data.userId) !== 42) {
    console.error("unexpected connected payload", JSON.stringify(data));
    clearTimeout(timer);
    socket.close();
    process.exit(1);
  }
  console.log("PASS connected userId=" + data.userId);
});

socket.on("feature-status-changed", (data) => {
  const payload = data?.data ?? data;
  if (Number(payload?.id) !== 1001 || payload?.rgb !== "G") {
    console.error("unexpected feature payload", JSON.stringify(data));
    clearTimeout(timer);
    socket.close();
    process.exit(1);
  }
  console.log("PASS feature id=" + payload.id + " rgb=" + payload.rgb);
  clearTimeout(timer);
  socket.close();
  process.exit(0);
});
`

	cmd := exec.Command(node, "--input-type=module", "-e", script)
	cmd.Dir = clientDir
	cmd.Env = append(os.Environ(), "WS_URL="+srv.URL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start node client: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()
	go func() {
		slurp, _ := io.ReadAll(stderr)
		if len(slurp) > 0 {
			t.Logf("node stderr: %s", slurp)
		}
	}()

	connected := false
	feature := false
	scanner := bufio.NewScanner(stdout)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for scanner.Scan() {
			line := scanner.Text()
			t.Log(line)
			if strings.Contains(line, "PASS connected") {
				connected = true
				h.BroadcastFeatureStatus(map[string]any{"id": float64(1001), "rgb": "G"})
			}
			if strings.Contains(line, "PASS feature") {
				feature = true
			}
		}
	}()

	select {
	case err := <-errCh:
		<-readDone
		if err != nil {
			t.Fatalf("socket.io-client failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("timed out waiting for official socket.io-client")
	}

	if !connected || !feature {
		t.Fatalf("connected=%v feature=%v", connected, feature)
	}
}

func nodeExecutable(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	return node
}

func frontendDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// tests/websocket-gateway/internal/hub -> ../../../../.. = metarang/
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "reactjs-frontend"))
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("frontend dir %s: %v", dir, err)
	}
	return dir
}
