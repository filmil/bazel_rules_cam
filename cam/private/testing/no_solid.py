# SPDX-License-Identifier: Apache-2.0
"""A test part with no solid, only a face: a job must refuse it."""

import os

import Part

Part.makePlane(10, 10).exportStep(os.environ["STEP_OUT"])
