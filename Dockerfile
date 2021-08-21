FROM alpine
ADD bin/clipboard /usr/local/bin
ENTRYPOINT ["clipboard"]