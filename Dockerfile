FROM ubuntu:14.04

ARG DT_API_URL="https://vzb12882.live.dynatrace.com/api"
ARG DT_API_TOKEN="5WUwr7a7TtOG4hSe_BC70"
ENV DT_HOME="/opt/dynatrace/oneagent"

RUN  apt-get update \
  && apt-get install -y wget openssh-client unzip \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p "$DT_HOME" && \
    wget -O "$DT_HOME/oneagent.zip" "$DT_API_URL/v1/deployment/installer/agent/unix/paas/latest?Api-Token=$DT_API_TOKEN" && \
    unzip -d "$DT_HOME" "$DT_HOME/oneagent.zip" && \
    rm "$DT_HOME/oneagent.zip"

WORKDIR /
# Now just add the binary
COPY prognosis /
#COPY cacert.pem /
EXPOSE 8001

ENTRYPOINT ["/opt/dynatrace/oneagent/dynatrace-agent64.sh"]
CMD ["/prognosis" ]
