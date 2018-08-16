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

RUN wget -O /usr/local/share/ca-certificates/sbsapko.crt http://pko.standardbank.co.za/05766pkojnb0001_Standard%20Bank%20ROOT%20CA.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca11.crt http://pko.standardbank.co.za/05766pkojnb0011_Standard%20Bank%20Policy%20CA%2011.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca21.crt http://pko.standardbank.co.za/05766pkojnb0021_Standard%20Bank%20Policy%20CA%2021.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca111.crt http://pko.standardbank.co.za/05766PKOJNB0111.sbicdirectory.com_Standard%20Bank%20CA%20111.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca112.crt http://pko.standardbank.co.za/05766PKOJNB0112.sbicdirectory.com_Standard%20Bank%20CA%20112.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca113.crt http://pko.standardbank.co.za/05766PKOJNB0113.sbicdirectory.com_Standard%20Bank%20CA%20113.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca114.crt http://pko.standardbank.co.za/05766PKOJNB0114.sbicdirectory.com_Standard%20Bank%20CA%20114.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca211.crt http://pko.standardbank.co.za/05766PKOJNB0211.corpdirectory.com_Standard%20Bank%20Certificate%20Authority%20211.crt && \
    wget -O /usr/local/share/ca-certificates/sbsaca212.crt http://pko.standardbank.co.za/05766PKOJNB0212.corpdirectory.com_Standard%20Bank%20Certificate%20Authority%20212.crt && \
    update-ca-certificates

WORKDIR /
# Now just add the binary
COPY prognosis /
EXPOSE 8001

ENTRYPOINT ["/opt/dynatrace/oneagent/dynatrace-agent64.sh"]
CMD ["/prognosis" ]
