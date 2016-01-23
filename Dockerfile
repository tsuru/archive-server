# Copyright 2016 Globo.com. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM alpine:3.2
ADD archive-server /bin/archive-server
ENTRYPOINT ["/bin/archive-server"]
