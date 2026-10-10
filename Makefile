# Test targets. The factory suite loads every binary under testdata/ and is
# run under a memory cap: a defective parser can turn a bad symbol table into
# a multi-gigabyte allocation (it took the machine down once).

MEM      ?= 4G
PARALLEL ?= 4
TESTBIN  := .tmp/ecusymbol.test
UPSTREAM := .tmp/upstream
RUN      := $(TESTBIN) -test.parallel $(PARALLEL)
CAP      := systemd-run --user --scope -q -p MemoryMax=$(MEM) -p MemorySwapMax=0 --working-directory=$(CURDIR)

.PHONY: test unit factory factory-t5 factory-t7 factory-t8 factory-upstream vet clean

## test: unit tests plus the factory suite
test: unit factory

## unit: everything except the factory suite (no binaries needed)
unit:
	go test -skip 'TestFactory' ./...

$(TESTBIN): $(wildcard *.go) $(wildcard */*.go)
	@mkdir -p .tmp
	go test -c -o $@ .

## factory: all factory binaries under testdata/
factory: $(TESTBIN)
	$(CAP) $(RUN) -test.run 'TestFactory'

## factory-t5 / -t7 / -t8: one ECU generation
factory-t5 factory-t7 factory-t8: factory-%: $(TESTBIN)
	$(CAP) $(RUN) -test.run 'TestFactory.*/$(shell echo $* | tr a-z A-Z)/'

## factory-upstream: the same suite against a fresh download of the binaries
factory-upstream: $(TESTBIN)
	@if [ ! -d $(UPSTREAM)/T5Binaries ]; then \
		rm -rf $(UPSTREAM) && \
		git clone -q --filter=blob:none --no-checkout https://github.com/mattiasclaesson/TuningSuites $(UPSTREAM) && \
		git -C $(UPSTREAM) sparse-checkout set --no-cone T5Binaries T7Binaries T8Binaries && \
		git -C $(UPSTREAM) checkout -q; \
	fi
	$(CAP) --setenv=ECUSYMBOL_TESTDATA=$(CURDIR)/$(UPSTREAM) $(RUN) -test.run 'TestFactory'

vet:
	go vet ./...

clean:
	rm -rf .tmp
