#!/bin/bash
date
set -x

# To avoid execution without passing environment variables
if [[ (-z "$NUMBER_OF_REQUESTS") || (-z "$FILE_NAME") || (-z "$FUNCTION_TARGET_IP") || (-z "$REPO_PATH") ]];
then
  echo -e "${RED}NUMBER_OF_REQUESTS AND/OR FILE_NAME AND/OR FUNCTION_TARGET_IP AND/OR REPO_PATH MISSING: workload.sh${NC}"
  exit
fi

mkdir -p ${REPO_PATH}
echo -e "status;latency" > ${FILE_NAME}
for i in `seq 1 ${NUMBER_OF_REQUESTS}`
do
    curl -X GET -o /dev/null -s -w '%{http_code};%{time_total}\n' ${FUNCTION_TARGET_IP}:8080 >> ${FILE_NAME}
done

sed -i 's/,/./g' ${FILE_NAME}
sed -i 's/;/,/g' ${FILE_NAME}
