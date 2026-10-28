#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work_dir="$(dirname "$repo_dir")"
onnx_dir="${ONNX_DIR:-$work_dir/go-build-deps/onnxruntime-linux-x64-1.21.0}"
cpp_dir="${CPP_LIB_DIR:-$work_dir/cpp-debs/sysroot/usr/lib/x86_64-linux-gnu}"
compose_dir="${COMPOSE_DIR:-/mnt/data/docker-apps/xiaozhi-go/src/docker/docker-composer}"
image="xiaozhi-mcp-media-go:local"

for path in "$onnx_dir/include" "$onnx_dir/lib" "$cpp_dir" "$repo_dir/lib/ten-vad/lib/Linux/x64" "$compose_dir/docker-compose.yml"; do
	if [[ ! -e "$path" ]]; then
		echo "缺少构建依赖或 Compose 文件: $path" >&2
		exit 1
	fi
done

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

export CGO_CFLAGS="-I$onnx_dir/include -I$repo_dir/lib/ten-vad/include"
export CGO_LDFLAGS="-L$onnx_dir/lib -lonnxruntime -L$repo_dir/lib/ten-vad/lib/Linux/x64 -L$cpp_dir"
export LD_LIBRARY_PATH="$onnx_dir/lib:$repo_dir/lib/ten-vad/lib/Linux/x64:$cpp_dir${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

cd "$repo_dir"
go test ./internal/domain/asr/funasr ./internal/app/server/chat ./internal/domain/play_music
go build -o "$build_dir/xiaozhi_server" ./cmd/server/
cat > "$build_dir/Dockerfile" <<'EOF'
FROM xiaozhi-pr1-main:1123125
COPY --chmod=755 xiaozhi_server /workspace/bin/xiaozhi_server
EOF
docker build -t "$image" "$build_dir"

cd "$compose_dir"
docker compose config -q
if ! docker compose config --format json | python3 -c 'import json,sys; assert json.load(sys.stdin)["services"]["main-server"]["image"] == "xiaozhi-mcp-media-go:local"'; then
	echo "Compose 的 main-server 镜像必须设为 $image" >&2
	exit 1
fi
docker compose up -d --force-recreate --no-deps main-server
docker ps --filter name=^/xiaozhi-main-server$ --format '{{.Names}} {{.Status}} {{.Image}}'
