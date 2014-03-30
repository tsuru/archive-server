#archive-server

archive-server is a daemon for serving git archives over HTTP. It contains two
APIs: one that generates archives on demmand, and the other that serves the
archives. Each archive is uniquely identified by a hash.

##Usage

	% archive-server -read-http 0.0.0.0:3232 -write-http 127.0.0.1:3131

This command will start the "administrative" service at 127.0.0.1:3131 and the
public service at 0.0.0.0:3232.
