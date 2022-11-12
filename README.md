This repository contains scripts and programs for digitizing paper documents and managing documents. It is highly
specific to my particular workflow, but feel free to adapt the code to your own needs. No issues, no pull requests, no
support, only code.

# Digitizing documents
## Acquiring masters
Using a Brother ADS-2400N connected via USB, we batch scan pages using

```sh
scanimage \
  -d 'brother5:bus1;dev1' \
  --format=tiff \
  # produce correct lexicographic sorting
  --batch="$(date -Iseconds)-page%05d.tif" \
  --mode "24bit Color" \
  --resolution 600 \
  # There are "left aligned" and "center aligned" sources. Using the center ones causes segfaults, and using the left ones produces correct, center-aligned results
  --source "Automatic Document Feeder(left aligned,Duplex)" \
  --MultifeedDetection=yes \
  --AutoDocumentSize=yes \
  # brscan's auto deskew isn't working well enough, so we'll do our own deskewing
  --AutoDeskew=no \
  # produce exactly two files per scanned sheet
  --SkipBlankPage=no
```
This produces two TIFF files per sheet of paper, with minimal processing done on the image data.

Our stacks of sheets are ordered newest to oldest, with multi-sheet documents ordered page 1 to page N. The ADF expects
sheets face down, top of the page at the bottom, scanning the sheet closest to the tray first.

If we take our stack and put it in the ADF as expected (possibly split several times if we have too many sheets to fit),
then the resulting files will sort such that in ascending order, the first file corresponds to the top of the stack and
the last file (file N) to the bottom of the stack. If the stack later grows as new documents arrive, scanning the
additions in the expected order will break the total sort. The first file will be the top of the initial stack, file N+1
will be the top of the addition, and the last file will be the bottom of the addition. However, multi-sheet documents
will continue to sort correctly, as long they didn't span multiple additions.

If we instead rotate the stack so that the sheets are facing us, we will scan from the back to the front of the stack.
If we sort the resulting files in descending order, they will match the order in the folder. This continues to work even
if we scan additions to the stack. The only downside is that in duplex scanning, the front and back page are now
swapped, but that's easy to account for. In theory we still want to place sheets so that the top of the sheet is at the
bottom, otherwise the documents will be upside down. However, as that's unintuitive, we'll rather fix the rotation after
scanning:


```sh
ls *.tif | xargs -t -I @ -n 1 -P 32 mogrify -rotate 180 @
```

We use our `merge_duplex` utility to merge the two files per sheet into a single multi-page TIFF. This
also compresses the files.

The output of these steps constitutes our master copies. We will archive these outside our DMS. In the following
steps we will produce access copies.

## Producing access copies

We process our scanned documents to remove blank pages, deskew them and potentially apply other filters. The result is
turned into PDF files containing TIFF images. We don't use multi-page TIFF here because it's poorly supported by most
software. A lot of image viewers will only show the first page. This is fine for the master copies, which we process
explicitly and directly, but it's not ideal for the access copies, which will be stored in our DMS, may be attached to
emails, etc.

We don't currently bother with binarizing the scanned documents or changing the white levels or anything like that. For
white paper sheets, the scanner does a good job picking the white point. For odd colors, like the dithered pink of a
prescription, we can't trivially make things look good automatically. It'd be nice to have a white background for
recycled, brownish paper, but I'm not sure that can be done fully automatically without a human deciding what looks good
for each individual sheet. We don't intend to print our documents en masse, so as long as they look readable on a
monitor, they're fine. We're not making copies of copies.

```sh
mkdir processed
for x in *.tif; do
  rm -rf tmp
  mkdir tmp
  tiffsplit $x tmp/
  for y in tmp/*.tif; do
	# Delete blank pages
	if ! isblank $y; then
	  rm $y;
	  continue
	fi

	# Deskew
	deskew -o tmp/deskewed.tif -b FFFFFF -c tdeflate $y
	mv -f tmp/deskewed.tif $y
  done

  # XXX handle the case where all pages were empty. all pages being empty is unlikely, so assume we have false positives in isblank, and include all pages.
  tiffcp tmp/* tmp/$x.tif
  tiff2pdf -z -o processed/$x.pdf tmp/$x.tif
done
rm -rf tmp
```

## Checking the results

In the next step, we look at each produced PDF and apply the following modifications:

- Merge two PDFS into one. This is used for documents consisting of multiple sheets.
- Delete pages. This is used to delete blank pages that weren't detected, and to delete worthless pages, like generic
  address information printed on the backs of sheets.
- Delete documents. Some documents that were filed are simply useless.

We do this using custom keybindings in zathura, which invoke the `mark-merge-document` script, which writes a log of
actions to `~/.marked_documents`. The `merge_documents` tool takes this log and executes it.

We use `for x in *.pdf(On); do zathura "$x" &; sleep 0.1; done` to open one instance of zathura per processed PDF. The
sleep is to work around the raciness of X11 - we want the opened windows (which are tabs in our i3 config) to be in the
same order as the files. `*.pdf(On)` is a zsh-specific kind of glob, which sorts the glob's results by name, in
descending order.

```
map <C-m> exec 'mark-merge-document base "$FILE"'
map <C-n> exec 'mark-merge-document merge "$FILE"'
map <C-b> exec 'mark-merge-document delpage "$FILE" "$PAGE"'
map <C-d> exec 'mark-merge-document delete "$FILE"'
```

```sh
#!/bin/sh
printf "$1\t$2\t$3\n" >> ~/.marked_documents
```

## Archiving our results

We feed the output of the previous step into our DMS, [Docspell](https://docspell.org/). Docspell takes care of OCR,
extracting dates and correspondents, and building an index. After we've uploaded the files, we delete them from the file
system. The master copies are kept in the file system, the access copies are kept in Docspell. Docspell keeps an
untouched version of the file we feed it, as well as a post-processed file, which in our case primarily means a file
converted to JPEG. The post-processed file is that the UI shows by default.
