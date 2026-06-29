// clone 2 creates a new child process similar to fork
// caller can control whether or not two processes share virtual addr space
// table fo fd and of signal handlers.
// sys calls allow new child process to be placed in separate ns(7)

// glibc clone() wrapper fn and underlying syscall
//
// clone() wrapper fn: when child process is created with
// clone wrapper fn
// when fn(arg) returns, child p terminates
// stack arg specifies location of stack used by child process
//
// calling process must setup memory space for child stack
//
// tid=thread identifier, tls=thread-local-storage
// ignoring the rest of clone3 for now
// fork then execve

// needed macro to tell glibc header
// which set of declarations to expose
#define _GNU_SOURCE
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/wait.h>
#include <unistd.h>

static int child(void *arg) {
	sethostname("in-clone", 8);
	char *args[] = {"/bin/bash", NULL};
	execve(args[0], args, NULL);
	return 1;
}

int main(void) {
	char *stack = malloc(1024 * 1024);

	pid_t pid = clone(child, stack + 1024 * 1024, CLONE_NEWUTS | SIGCHLD, NULL);
	if (pid == -1) {
		perror("clone");
	}
	waitpid(pid, NULL, 0);
	free(stack);
	return 0;
}
