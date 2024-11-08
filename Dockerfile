FROM debian:buster as builder

# 构建容器的时候就不负责编译了，编译放在github action的阶段中，这里就只把编译好的可执行文件搞进来运行就可以
RUN mkdir -p /app
COPY ./huoxian-crawler-whois /app
# 不确定会不会可执行，先给它个可执行权限再说...
RUN chmod u+x /app/huoxian-crawler-whois
WORKDIR /app

VOLUME /app/data