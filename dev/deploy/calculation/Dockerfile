FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends libreoffice-calc python3-uno && rm -rf /var/lib/apt/lists/*
COPY calculate.py /opt/keel/calculate.py
ENTRYPOINT ["/usr/bin/python3", "/opt/keel/calculate.py"]
