# Presentation Overview

The presentation time is 25 minutes, with 5 sections or parts the 2nd
headers below represent each part.

## Containers Importance in Cloud Computing (5 min)

Every cloud service is containerized, or virtualized over servers.
The higher level products like EC2, ECS, EKS, S3, VPC, microservices, distributed
computing in general, CDNs, edge compute, creating high redudency, cache, all rely on containerization
and virtualization in some way. No one runs a bare server with one OS to run all these things,
excepting MAYBE pet projects or home servers.

Even things that don't appear to be "containerized" are.

## What is a Container Runtime (5 min)

Go into the different parts of a tool like Docker or Podman,
image creation, lifecycle orchestration.

## The Mental Model of processes and making Sys Calls to the Kernel (5 min)

The claw metaphor from Toy Story. "The Kernel"!

## Implementing the OCI Runtime CLI (5 min)

OCI walkthrough and coding

## Running a nodejs app and finishing up (5 min)

Show the nodejs application serving a "Hello CYC" web page

References to material used for boxes and presentation.
Where to learn more:

- [Podman GH](https://github.com/podman-container-tools/podman)
- [Containerd GH](https://github.com/containerd/containerd)
- [Liz Rice Website](https://www.lizrice.com/)
