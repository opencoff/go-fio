

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
testdir = ../tests
tests = $(wildcard $(testdir)/*.t)

# controls whether trunner runs tests in parallel or
# serially. Default for now is to make it serial - until we
# iron out all issues
# TODO Remove this when we pass all tests
serial = -s

# command line option for make
ifeq ($(CONCURRENT),1)
	serial :=
endif

.PHONY: clean $(trunner)

all: $(trunner)

$(trunner):
	$(build) -b $(bindir) .:$(program)


ifeq ($(tests),)
test: $(trunner)
	@echo "no tests in $(testdir)"
else
test: $(trunner)
	$(trunner) --log-stdout -p $(serial) $(tests)
endif

clean:
	-rm -f $(trunner)


# vim: noexpandtab:ts=4:sw=4:
