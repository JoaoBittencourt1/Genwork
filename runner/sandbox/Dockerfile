# The sandbox every bot run executes in: Git, the provider CLIs, and common toolchains. CLI versions are pinned here and
# nowhere else (rule 7); the labels tell the runner which CLIs and versions this image provides.
FROM golang:1.26-bookworm AS go

FROM node:22-bookworm-slim

ARG VERSION
ARG CLAUDE_CODE_VERSION=2.1.318
ARG CODEX_VERSION=0.166.0
ARG GROK_CLI_VERSION=0.0.34
ARG CURSOR_AGENT_INSTALLER=https://cursor.com/install

RUN test -n "${VERSION}" || { echo "the VERSION build argument is required (rule 50)" >&2; exit 1; }

RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates curl openssh-client python3 python3-pip python3-venv build-essential jq ripgrep \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

RUN npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" "@openai/codex@${CODEX_VERSION}" "@vibe-kit/grok-cli@${GROK_CLI_VERSION}" \
    && npm cache clean --force

# The Cursor agent ships through an installer script that cannot be pinned, so it is installed into its own directory,
# which any user can read, and linked onto the PATH.
RUN mkdir -p /opt/cursor \
    && HOME=/opt/cursor curl -fsSL "${CURSOR_AGENT_INSTALLER}" | HOME=/opt/cursor bash \
    && ln -s "$(ls /opt/cursor/.local/share/cursor-agent/versions/*/cursor-agent | head -1)" /usr/local/bin/cursor-agent \
    && chmod -R a+rX /opt/cursor \
    && cursor-agent --version

LABEL org.opencontainers.image.title="genwork-sandbox" \
      org.opencontainers.image.version="${VERSION}" \
      genwork.tool.claude="${CLAUDE_CODE_VERSION}" \
      genwork.tool.codex="${CODEX_VERSION}" \
      genwork.tool.grok="${GROK_CLI_VERSION}" \
      genwork.tool.cursor-agent="latest"

# Runs unprivileged with a temporary home: the runner starts the container with its own uid, a read-only root file
# system, and tmpfs for /tmp and $HOME.
WORKDIR /workspace
ENV HOME=/home/sandbox
CMD ["sh", "-c", "echo 'genwork-sandbox: the runner starts a CLI here' >&2; exit 64"]
