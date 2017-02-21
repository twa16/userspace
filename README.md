# Userspace

### What is Userspace?
Userspace is a project that was envisioned by Manuel Gauto 
in 2014 during a meeting of George Mason University's Student Run Computing and Technology (http://srct.gmu.edu).
The idea was to create a project that allowed members to spin up small VMs that they could do
scratch work on for their project. The original project called for VMs but now that Docker has
matured, it is a much better fit.

### Goals

- SSH Access into Spaces
- Limited Lifetime
  * Pauses container on inactivity
  * Deleted after a certain period of inactivity(No SSH or Network traffic)
- Authentication using University credentials over LDAP or CAS
- Metrics
  * Disk Usage
  * Network Usage
    * Throughput
    * Destinations
  * Session Tracking
    * SSH Tracking
    * Network Session Tracking

### Architecture
Web Interface --> UserSpace Daemon --> Docker Hosts

The Userspace Daemon will connect to one or more Docker hosts and manage the containers
on there. The Daemon will expose the various functions of the system over a REST API. 

### API
The API is under active development. 
You can see the documentation here: https://userspace.restlet.io
This swagger.yaml file has the most update to date version of the API. 
The online viewer will be updated periodically.

### Space Creation Process

1. User requests a Space. This gives us: Name and Image
2. The Daemon chooses a Host. This gives us: An external address
3. A port is chosen
4. A proxy entry is created

### What do you get in a Space

**So what is a space?**
A Space is a small linux environment that lives inside of a Docker container. The key
features are as follows:

- You get root SSH access to the Space using the specified port and address
- You will get an external port that is pointed at port 1337 on the Space. You service should listen here.
