FROM adoptopenjdk/openjdk11:jdk-11.0.2.9-slim as builder

RUN apt-get update \
    && apt-get install -y apt-utils curl sudo unzip \
    && useradd -m app && echo "app:app" | chpasswd && adduser app sudo \
    && mkdir -p /opt/ && cd /opt/ \
    && echo "Downloading gradle.." \
    && curl -SfL https://services.gradle.org/distributions/gradle-5.3-bin.zip -o gradle-bin.zip \
    && unzip gradle-bin.zip -d /opt/ \
    && rm gradle-bin.zip

# Export some environment variables
ENV GRADLE_HOME=/opt/gradle-5.3/
ENV PATH=$PATH:$GRADLE_HOME/bin

RUN mkdir -p /home/app/libs

ENV GRADLE_OPTS="-Dorg.gradle.daemon=false"
WORKDIR /home/app

COPY . /home/app/

RUN gradle build --stacktrace

FROM adoptopenjdk/openjdk11:jdk-11.0.2.9-slim as ship
RUN apt-get update \
    && apt-get install -y apt-utils curl unzip bash build-essential \
    && echo "Pulling watchdog binary from Github." \
    && curl -sSL https://github.com/openfaas-incubator/of-watchdog/releases/download/0.4.6/of-watchdog > /usr/bin/fwatchdog \
    && chmod +x /usr/bin/fwatchdog

RUN useradd -m app && echo "app:app" | chpasswd && adduser app sudo
WORKDIR /home/app
COPY --from=builder /home/app/entrypoint/build/distributions/entrypoint-1.0.zip ./entrypoint-1.0.zip
RUN unzip ./entrypoint-1.0.zip

RUN chown -R app /home/app
USER app

ENV entrypoint_port="8082"
ENV upstream_url="http://127.0.0.1:8081"
ENV mode="http"
ENV CLASSPATH="/home/app/entrypoint-1.0/lib/*"
ENV image_url="http://s3.amazonaws.com/wallpapers2/wallpapers/images/000/000/408/thumb/375.jpg?1487671636"
ENV scale="0.1"

RUN mkdir "/home/app/gci"
COPY gci/* /home/app/gci/
ENV PATH_TO_GCI="/home/app/gci"
RUN bash ${PATH_TO_GCI}/libgcso-entrypoint.sh

#ENV fprocess="taskset 0x1 java -server -Xms128m -Xmx128m -cp /home/app/entrypoint-1.0/lib/* -XX:+UnlockExperimentalVMOptions -XX:+UseG1GC -XX:G1NewSizePercent=90 -XX:G1MaxNewSizePercent=90 -Xlog:gc:file=/home/app/gc_thumb.log -Djvmtilib=/home/app/gci/libgc.so -javaagent:/home/app/gci/gciagent-0.1-jar-with-dependencies.jar=8500 com.openfaas.entrypoint.App"
EXPOSE 8080

HEALTHCHECK --interval=2s CMD [ -e /tmp/.lock ] || exit 1

#CMD ["sh","-c","taskset 0x2 nohup ./gci/gci-proxy --port=8081 --target=127.0.0.1:8082 --gci_target=127.0.0.1:8500 --ygen=104857600 >/home/app/proxy-stdout.log 2>/home/app/proxy-stderr.log & taskset 0x2 fwatchdog"]
CMD ["sh","-c","taskset 0x1 nohup java -server -Xms128m -Xmx128m -XX:+UnlockExperimentalVMOptions -XX:+UseG1GC -XX:G1NewSizePercent=90 -XX:G1MaxNewSizePercent=90 -Xlog:gc:file=/home/app/gc_thumb.log -Djvmtilib=/home/app/gci/libgc.so -javaagent:/home/app/gci/gciagent-0.1-jar-with-dependencies.jar=8500 com.openfaas.entrypoint.App & taskset 0x2 ./gci/gci-proxy --port=8080 --target=127.0.0.1:8082 --gci_target=127.0.0.1:8500 --ygen=104857600 >/home/app/proxy-stdout.log 2>/home/app/proxy-stderr.log"]
