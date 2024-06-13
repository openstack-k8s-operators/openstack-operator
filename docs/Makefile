BUILD = upstream
BUILD_DIR = ../docs_build
ROOTDIR = $(realpath .)
NAME = ctlplane
DEST_DIR = $(BUILD_DIR)/$(NAME)
DEST_HTML = $(DEST_DIR)/index-$(BUILD).html
DEST_PDF = $(BUILD_DIR)/$(NAME)-$(BUILD).pdf
IMAGES_DIR = $(DEST_DIR)/images
IMAGES_TS = $(DEST_DIR)/.timestamp-images
MAIN_SOURCE = user.adoc
OTHER_SOURCES = $(shell find ./assemblies -type f)
IMAGES = $(shell find ./images -type f)
ALL_SOURCES = $(MAIN_SOURCE) $(OTHER_SOURCES) $(IMAGES)
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

html-latest: prepare $(IMAGES_TS) $(DEST_HTML)

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
	${BROWSER_OPEN} "file://$(realpath $(ROOTDIR)/$(DEST_HTML))"

open-pdf: pdf
	${BROWSER_OPEN} "$(realpath $(ROOTDIR)/$(DEST_PDF))"

$(IMAGES_TS): $(IMAGES)
	cp $? $(IMAGES_DIR)
	touch $(IMAGES_TS)

$(DEST_HTML): $(ALL_SOURCES)
	$(BUNDLE_EXEC) asciidoctor -a source-highlighter=highlightjs -a highlightjs-languages="yaml,bash" -a highlightjs-theme="monokai" -a build=$(BUILD) -b xhtml5 -d book -o $@ $(MAIN_SOURCE)

$(DEST_PDF): $(ALL_SOURCES)
	$(BUNDLE_EXEC) asciidoctor-pdf -a build=$(BUILD) -d book -o $@ $(MAIN_SOURCE) $(IMAGES)
