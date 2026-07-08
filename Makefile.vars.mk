IMG_TAG ?= latest

CURDIR ?= $(shell pwd)
BIN_FILENAME ?= cupel

# Image URL to use all building/pushing image targets
GHCR_IMG ?= ghcr.io/helmetica-framework/cupel:$(IMG_TAG)
