FROM python:2

WORKDIR /root/kismatic
EXPOSE 8080 8443

ADD out-docker/ /root/kismatic
RUN chmod +x /root/kismatic/*

ENTRYPOINT ["/root/kismatic/kismatic"]
CMD ["--help"]
