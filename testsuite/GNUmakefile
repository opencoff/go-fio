

# build tool
build   = ./build

# the  program to be built
program  = tdircmp

# top of the dir for $(program); the actual binary
# is placed in $(bindir)/$(arch)
bindir  = ./bin
arch    = $(shell $(build) --print-arch)

# full name of the program
trunner = $(bindir)/$(arch)/$(program)

# tests we will run
testdir = ./tests
tests = $(wildcard $(testdir)/*.t)

# controls whether trunner runs tests in parallel or
# serially. Default for now is to make it serial - until we
# iron out all issues
# TODO Remove this when we pass all tests
serial :=

# default concurrency is 8 threads;
concurrency := --concurrency 8

# by default we keep logs in a per-test-dir
log :=

# command line option for make
ifeq ($(SERIAL),1)
	serial = -s
endif

# if command line says V=1, put logs in stdout
ifeq ($(V),1)
	log = --log-stdout
endif

.PHONY: clean $(trunner)

all: $(trunner)

$(trunner):
	$(build) -b $(bindir) .:$(program)


ifeq ($(tests),)
tests test: $(trunner)
	@echo "no tests in $(testdir)"
else
tests test: $(trunner)
	$(trunner) $(log) -p $(concurrency) $(serial) $(tests)
endif

clean:
	-rm -f $(trunner)


# vim: noexpandtab:ts=4:sw=4:
