gocnc
=====

GCode sender and optimizer for CNC routers

This tool is written in golang, and as such requires a configured golang environment (https://golang.org/doc/install).

Install
====

      go get github.com/joushou/gocnc

Build
====

In the gocnc directory ($GOPATH/src/github.com/joushou/gocnc)

      go build

Run
====

The usage guide can be retrieved with:

      ./gocnc

A usage example:

      ./gocnc --input ~/gcode.nc --device /dev/tty.usbmodem1441

Detailed description
====

This tool has been written to assist me in my use of a Shapeoko 2, where I became
horribly frustrated with GCode generators' poor output, as well as GCode senders'
poor user experience and speed.

For the GCode sender, this tool implements a "streamer" for GRBL (more to come...?),
that ensures that the 127 char serial buffer is always as stuffed as it can be, so that
there will always be something for it to chew on. Otherwise, large amounts of very short
operations might cause its planning buffer to be depleted, stopping all movement until
more input has been processed.

For optimizations, I have gone to quite crazy lengths, implementing a sort of interpreter,
or "CNC VM". It "executes" the entire parsed AST, updating its position stack and states
along the way. When done, the stack is dumped, which makes optimizing the code much easier,
as all states have been kept track of. Working on the AST/file directly, comes with the
risk of losing other flags that were on the same line, making modifications an utter
headache (Trust me, I speak from experience. That's what my first tool did.)

The optimization passes can be summarized as:
* Remove redundant code (Does not change behaviour)
* Use rapid moves Z-axis lift and drill moves where possible
* Group route operations, to minimize time spent seeking around

The last is by far the most complicated, and results in the largest gain. The slower the
machine, the larger the gain. For my very fast shapeoko, I get ~15-20% speedup on the tests
I have made, which will become much more with more sane maximum speeds. It is only really
useful for 2D stuff, and automatically bails out with a warning when it might be unsafe to run.

To aid controllers like Grbl, and in general produce higher calculation accuracy,
arcs are calculated by the VM, so that the VM position stack only contains straight lines.
This makes optimization and analysis *much* easier, allows for double/float64 during
calculations, and lets a very heavy task off Grbl's shoulders. Many GCode interpreters
seem to be unable to handle the more complicated uses of arcs as well, and this ensures
that they don't have to worry about that headache.

In the future, more functionality will be soft-implemented, such as peck drilling cycle, etc.

