FROM sonarsource/sonar-scanner-cli:12.0.0.3214_8.0.1 AS sonar

FROM mcr.microsoft.com/devcontainers/go:1.25-bookworm
ARG TARGETPLATFORM
COPY --from=sonar /opt/sonar-scanner/lib/sonar-scanner-cli-8.0.1.6346.jar /usr/local/lib/
COPY --from=sonar /opt/sonar-scanner/bin/sonar-scanner /usr/local/bin/

RUN apt update \
 && apt install -y \
    awscli \
    openjdk-17-jre \
    vim \
    python3-pip \
    ca-certificates curl gnupg npm \
 && npm install -g @fission-ai/openspec@latest \
 && npm i -g opencode-ai \
 && pip install --upgrade bump2version openviking --break-system-packages \
 && go install github.com/t-yuki/gocover-cobertura@latest \
 && go install gitlab.com/fgmarand/gocoverstats@latest \
 # CleanUp
 && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*
RUN echo "source ~/.alias" >> /root/.bashrc \
 && echo "source ~/.alias" >> /root/.zshrc
COPY <<-"EOT" /root/.alias
 alias gs='git status'
 alias gb='git branch'
 alias gd='git diff'
 alias gco='git checkout'
EOT
RUN <<eot
   apt-get install ca-certificates curl gnupg lsb-release
   curl -fsSL https://azlux.fr/repo.gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/azlux-archive-keyring.gpg
   echo \
   "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/azlux-archive-keyring.gpg] http://packages.azlux.fr/debian \
   $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/azlux.list >/dev/null
   apt-get update
   apt-get install -y docker-ctop
eot
RUN <<eot
   cd /usr/bin
   if [ "${TARGETPLATFORM}" = "linux/amd64" ]; then
      wget -qO - https://github.com/go-task/task/releases/download/nightly/task_linux_amd64.tar.gz |tar xfz - task
      wget -qO /usr/local/bin/mc https://dl.min.io/client/mc/release/linux-amd64/mc
      chmod +x /usr/local/bin/mc
   elif [ "${TARGETPLATFORM}" = "linux/arm64" ]; then
      wget -qO - https://github.com/go-task/task/releases/download/nightly/task_linux_arm64.tar.gz |tar xfz - task
      wget -qO /usr/local/bin/mc https://dl.min.io/client/mc/release/linux-arm64/mc
      chmod +x /usr/local/bin/mc
   fi
eot
#RUN apt install -y neo4j-client
#RUN pipx install sqlit-tui
COPY homelab-rootCA.pem /usr/local/share/ca-certificates/homelab-rootCA.crt
RUN update-ca-certificates
ENV OPENVIKING_CLI_CONFIG_FILE=/Users/christian.kniep/.config/openviking/ovcli.conf