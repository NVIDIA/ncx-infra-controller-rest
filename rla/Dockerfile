# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# This is the old Dockerfile that can be used for quick local builds.  See ../docker/production/Dockerfile.carbide-rla for the real one.

FROM alpine

# Libraries needed for go binaries
RUN apk add libc6-compat

COPY build/rla /opt/rla/rla

EXPOSE 50051

CMD /opt/rla/rla serve
