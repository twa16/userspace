# Userspace

### What is Userspace?
Userspace is a project that was envisioned by Manuel Gauto 
in 2014 during a meeting of George Mason University's Student Run Computing and Technology (http://srct.gmu.edu).
The idea was to create a project that allowed members to spin up small VMs that they could do
scratch work on for their project. The original project called for VMs but not that Docker has
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