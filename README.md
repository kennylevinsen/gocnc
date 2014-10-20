gocnc
=====

CNC tool in Go.

It optimizes the GCode received. When the original Python tool has been
completely ported (Python was a tad slow to deal with MeshCAM's multi-megabyte
files), it should quite consistently result in less than half the codesize
for MakerCAM, and sometimes 1/10th for Easel.
