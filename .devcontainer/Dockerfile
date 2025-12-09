FROM sonarsource/sonar-scanner-cli:12.0.0.3214_8.0.1 AS sonar
FROM mcr.microsoft.com/devcontainers/go:1-1.19-bullseye
COPY --from=sonar /opt/sonar-scanner/lib/sonar-scanner-cli-8.0.1.6346.jar /usr/local/lib/
COPY --from=sonar /opt/sonar-scanner/bin/sonar-scanner /usr/local/bin/

RUN apt update \
 && apt install -y \
    awscli \
    openjdk-11-jre \
    vim \
    python3-pip \
 # CleanUp
 && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*
RUN pip install --upgrade bump2version
RUN go install github.com/t-yuki/gocover-cobertura@latest \
 && go install gitlab.com/fgmarand/gocoverstats@latest
RUN echo "source ~/.alias" >> /root/.bashrc
RUN echo "source ~/.alias" >> /root/.zshrc
COPY <<-"EOT" /root/.alias
 alias gs='git status'
 alias gb='git branch'
 alias gd='git diff'
 alias gco='git checkout'
EOT
