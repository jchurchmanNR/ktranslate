FROM ubuntu:20.04
RUN apt-get update && apt-get install -y \
    krb5-user libsasl2-modules-gssapi-mit liblz4-dev libzstd-dev libsasl2-dev libpcap-dev
COPY bin/ktranslate /usr/bin/ktranslate
COPY config.json /etc/config.json
COPY code2city.mdb /etc/code2city.mdb
COPY code2region.mdb /etc/code2region.mdb
COPY udr.csv /etc/udr.csv
COPY lib/librdkafka.so.1 /lib/x86_64-linux-gnu/librdkafka.so.1
COPY mibs.db /etc/mibs.db
ENTRYPOINT ["/usr/bin/ktranslate", "-metalisten", "0.0.0.0:8083", "-listen", "0.0.0.0:8082", "-mapping", "/etc/config.json", "--city", "/etc/code2city.mdb", "--region", "/etc/code2region.mdb", "--udrs", "/etc/udr.csv"]

EXPOSE 8082
EXPOSE 8083