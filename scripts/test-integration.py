#!/usr/bin/env python3
"""Run the Go integration tests against two disposable local etcd servers."""
import contextlib
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time


def start_etcd(binary, directory, cleanup):
    with socket.socket() as client, socket.socket() as peer:
        client.bind(("127.0.0.1", 0))
        peer.bind(("127.0.0.1", 0))
        client_port = client.getsockname()[1]
        peer_port = peer.getsockname()[1]

    endpoint = f"http://127.0.0.1:{client_port}"
    peer_url = f"http://127.0.0.1:{peer_port}"
    log_path = directory / "etcd.log"
    with log_path.open("wb") as log:
        proc = subprocess.Popen(
            [
                binary,
                "--name=integration",
                f"--data-dir={directory / 'data'}",
                f"--listen-client-urls={endpoint}",
                f"--advertise-client-urls={endpoint}",
                f"--listen-peer-urls={peer_url}",
                f"--initial-advertise-peer-urls={peer_url}",
                f"--initial-cluster=integration={peer_url}",
            ],
            stdout=log,
            stderr=subprocess.STDOUT,
        )

    def stop():
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()

    cleanup.callback(stop)
    for _ in range(100):
        if proc.poll() is not None:
            break
        try:
            with socket.create_connection(("127.0.0.1", client_port), timeout=0.2):
                return endpoint
        except OSError:
            time.sleep(0.1)
    raise RuntimeError(log_path.read_text(errors="replace"))


def main():
    if len(sys.argv) != 2:
        sys.exit("Usage: python3 scripts/test-integration.py /path/to/etcd")
    binary = str(Path(sys.argv[1]).resolve(strict=True))
    with contextlib.ExitStack() as cleanup:
        first = Path(cleanup.enter_context(tempfile.TemporaryDirectory()))
        second = Path(cleanup.enter_context(tempfile.TemporaryDirectory()))
        env = dict(
            os.environ,
            ETCD_ENDPOINTS=start_etcd(binary, first, cleanup),
            ETCD_SECOND_ENDPOINT=start_etcd(binary, second, cleanup),
        )
        return subprocess.call(
            ["go", "test", "-race", "-tags", "integration", "-count=1", "-run", "^TestIntegration", "-v", ".", "./internal/etcd"],
            cwd=Path(__file__).resolve().parents[1],
            env=env,
        )


if __name__ == "__main__":
    sys.exit(main())
