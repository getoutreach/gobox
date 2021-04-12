#!/bin/bash

# These environment variables should allow a binary running on the host to
# behave properly and interact with services running in the k9s-based
# dev-environment so long as 'devenv tunnel' is running

export MY_NAMESPACE="gobox--bento1a"
export MY_SERVICE_ACCOUNT_NAME="gobox-svc"
