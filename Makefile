APP := gobox
OSS := true
_ := $(shell ./scripts/devbase.sh) 

include .bootstrap/root/Makefile

###Block(targets)
.PHONY: test-only
test-only::
	$(BASE_TEST_ENV) ./scripts/shell-wrapper.sh test.sh
###EndBlock(targets)
