---
theme: seriph
title: "boxes — Building a Minimal Container Runtime in Go"
info: |
  ## boxes
  A from-scratch, OCI-style Linux container runtime in Go.
  Conference talk by Michael Duren.
class: text-center
highlighter: shiki
lineNumbers: true
drawings:
  persist: false
transition: slide-left
mdc: true
---

# `boxes`

Building a minimal Linux container runtime in Go

<div class="abs-br m-6 text-sm opacity-60">
  Michael Duren · <a href="https://github.com/michael-duren/boxes" target="_blank">github.com/michael-duren/boxes</a>
</div>

<!--
Presenter notes: Welcome. Today we're going to take the magic out of containers
by building one. Not a toy that prints "hello", a containerized nodejs application
-->

---

# whoami

- Software Engineer, SRE, C enthusiast
- Wanted to understand what `docker run` _actually_ does
- So I built `box` — a small runtime in the spirit of `runc` and `youki`
- Today will be writing a **very** minimal runtime but it will illustrate the OS
  features that make containerization a possibility

::right::

<div class="pl-6 pt-12">

```text
┌──────────────┐
│   docker     │
├──────────────┤
│  containerd  │
├──────────────┤
│    runc      │ ◄── this layer
├──────────────┤
│ Linux kernel │
└──────────────┘
```

`box` lives where `runc` lives:
the thing that actually talks to the kernel.

</div>

<!--
The whole stack above runc is orchestration. The interesting, "what is a container"
part is this bottom layer. That's what we're rebuilding.
-->

---

# A container is not a thing, a process is though

There is no `container` syscall.

A "container" is just a normal Linux process with three things wrapped around it:

<v-clicks>

- **Namespaces** — what the process can _see_ (PIDs, mounts, network, hostname…)
- **cgroups** — what the process can _use_ (CPU, memory, pids, IO)
- **A root filesystem** — what `/` _is_ for that process

</v-clicks>

<div v-click class="pt-6 opacity-80">

That's it. Isolation is a lie we tell a perfectly ordinary process.

</div>

<!--
Key reframe for the audience. Once you see it this way, building a runtime stops
being scary — you're just assembling kernel primitives in the right order.
-->

---

# What I learned

<v-clicks>

- **Containers are assembly, not invention.** Every piece is a documented kernel feature.
- **The hard parts aren't isolation** — they're lifecycle, races, and "is it _really_ dead?"
- **The spec is your friend.** Implementing the OCI contract means free, upstream tests.
- **Re-exec yourself** is the trick that unlocks namespaced children.
- Reading `runc` / `youki` source is the best documentation there is.

</v-clicks>

---

# Thanks 📦

`box` — a minimal OCI runtime you can actually read

<div class="pt-4">

[github.com/michael-duren/boxes](https://github.com/michael-duren/boxes)

</div>

<div class="pt-10 opacity-70 text-sm">
Questions? Let's talk about the zombie-process problem.
</div>

<!--
Wrap up. Invite questions, point to the repo, mention it's beginner-readable on purpose.
-->
