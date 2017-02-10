FROM ubuntu:16.04

RUN \
  sed -i 's/# \(.*multiverse$\)/\1/g' /etc/apt/sources.list && \
  apt-get update && \
  apt-get -y upgrade && \
  apt-get install -y software-properties-common curl git man unzip vim wget openssh-server

# Start SSH on boot
RUN update-rc.d ssh defaults

# Set environment variables.
ENV HOME /root

# Define working directory.
WORKDIR /root

RUN echo "root:Docker!" | chpasswd

# Define default command.
CMD ["bash"]
