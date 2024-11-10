# go-fio - optimized file I/O routines

## Description
This is a collection of cross platform file I/O and File system functions. These use
concurrency to speed up the underlying functions.

- `fio.Info`: a serializable version of Stat/Lstat that uses OS specific functions
- Utilities to copy a file or dir along with all its metadata (including XATTR)
  The file copy functions will use the best underlying primitive like reflink(2);
  and fallback to using mmap based copy.
- `Safefile`: a wrapper over `os.OpenFile` that atomically commits the writes. This
  uses a temporary file for all the I/O and calling `Close()` atomically renames
  the temporary file to the intended file.


### Subpackages
- `cmp`: compares two directory trees and returns their differences
- `clone`: clones a source directory tree to a destination - skipping over identical
  files.
- `walk`: A concurrent directory tree traversal library
