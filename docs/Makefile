BUILD = upstream
BUILD_DIR = ../docs_build
ROOTDIR = $(realpath .)
NAME = openstack
DEST_DIR = $(BUILD_DIR)/$(NAME)
CTLPLANE_HTML = $(DEST_DIR)/ctlplane-$(BUILD).html
DATAPLANE_HTML = $(DEST_DIR)/dataplane-$(BUILD).html
DEST_PDF = $(BUILD_DIR)/$(NAME)-$(BUILD).pdf
IMAGES_DIR = $(DEST_DIR)/images
IMAGES_TS = $(DEST_DIR)/.timestamp-images
CTLPLANE_SOURCE = ctlplane.adoc
DATAPLANE_SOURCE = dataplane.adoc
OTHER_SOURCES = $(shell find ./assemblies -type f)
IMAGES = $(shell find ./images -type f)
ALL_SOURCES = $(CTLPLANE_SOURCE) $(OTHER_SOURCES) $(IMAGES)
UNAME = $(shell uname)
BUNDLE_EXEC ?= bundle exec

ifeq ($(UNAME), Linux)
BROWSER_OPEN = xdg-open
endif
ifeq ($(UNAME), Darwin)
BROWSER_OPEN = open
endif

all: html

html: html-latest

html-latest: prepare $(IMAGES_TS) $(CTLPLANE_HTML) $(DATAPLANE_HTML)

pdf: prepare $(DEST_PDF)

prepare:
	@mkdir -p $(BUILD_DIR)
	@mkdir -p $(DEST_DIR) $(IMAGES_DIR)

clean:
	@rm -rf "$(DEST_DIR)" "$(DEST_PDF)"

watch-html:
	@which inotifywait > /dev/null || ( echo "ERROR: inotifywait not found, install inotify-tools" && exit 1 )
	while true; do \
		inotifywait -r -e modify -e create -e delete .; \
		sleep 0.5; \
		$(MAKE) html; \
	done

open-html: html
	${BROWSER_OPEN} "file://$(realpath $(ROOTDIR)/$(CTLPLANE_HTML))"

open-pdf: pdf
	${BROWSER_OPEN} "$(realpath $(ROOTDIR)/$(DEST_PDF))"

$(IMAGES_TS): $(IMAGES)
	cp $? $(IMAGES_DIR)
	touch $(IMAGES_TS)

$(CTLPLANE_HTML): $(ALL_SOURCES)
	$(BUNDLE_EXEC) asciidoctor -a source-highlighter=highlightjs -a highlightjs-languages="yaml,bash" -a highlightjs-theme="monokai" -a build=$(BUILD) -b xhtml5 -d book -o $@ $(CTLPLANE_SOURCE)

$(DATAPLANE_HTML): $(ALL_SOURCES)
	$(BUNDLE_EXEC) asciidoctor -a source-highlighter=highlightjs -a highlightjs-languages="yaml,bash" -a highlightjs-theme="monokai" --failure-level WARN -a build=$(BUILD) -b xhtml5 -d book -o $@ $(DATAPLANE_SOURCE)

$(DEST_PDF): $(ALL_SOURCES)
	$(BUNDLE_EXEC) asciidoctor-pdf -a build=$(BUILD) -d book -o $@ $(CTLPLANE_SOURCE) $(IMAGES)
