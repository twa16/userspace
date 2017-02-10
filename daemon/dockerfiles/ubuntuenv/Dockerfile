FROM ubuntu:16.04

RUN \
  sed -i 's/# \(.*multiverse$\)/\1/g' /etc/apt/sources.list && \
  apt-get update && \
  apt-get -y upgrade && \
  apt-get install -y software-properties-common curl git man unzip vim wget openssh-server


#Setup SSH config
RUN service ssh stop
RUN rm /etc/ssh/sshd_config
COPY sshd_config /etc/ssh/
COPY sshd_banner /etc/ssh/

# Start SSH on boot
RUN update-rc.d ssh defaults
RUN update-rc.d ssh enable

# Set environment variables.
ENV HOME /root

# Define working directory.
WORKDIR /root

RUN echo "root:Docker!" | chpasswd

# Define default command.
CMD ["bash"]
