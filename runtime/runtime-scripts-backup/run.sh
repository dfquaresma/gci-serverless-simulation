#!/bin/bash
date
set -x

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "FLAGS: ${FLAGS:=gci nogci}"
echo "INITIAL_EXPID: ${INITIAL_EXPID:=1}"
echo "NUMBER_OF_EXPERIMENTS: ${NUMBER_OF_EXPERIMENTS:=1}"
echo "NUMBER_OF_REQUESTS: ${NUMBER_OF_REQUESTS:=5000}"
echo "FUNCTION_TARGET_IP: ${FUNCTION_TARGET_IP:=10.11.16.117}"
echo "FUNCTION_TARGET_PORT: ${FUNCTION_TARGET_PORT:=8080}"
echo "WORKLOAD_TARGET_IP: ${WORKLOAD_TARGET_IP:=10.11.16.128}"
echo "ID_RSA_PATH: ${ID_RSA_PATH:=id_rsa}"
echo "RESULTS_PATH: ${RESULTS_PATH:=/home/ubuntu/gci-faas-sim/experiment/results/}"
echo "LOCAL_RESULTS_PATH: ${LOCAL_RESULTS_PATH:=/home/davidfq/gci-faas-sim/experiment/results/}"
echo "CD_TO_SCRIPTS_PATH: ${CD_TO_SCRIPTS_PATH:=cd /home/ubuntu/gci-faas-sim/experiment}"

rm -rf ${LOCAL_RESULTS_PATH}

ssh-keygen -f "/home/ubuntu/.ssh/known_hosts" -R ${FUNCTION_TARGET_IP}
ssh -i ${ID_RSA_PATH} ubuntu@${FUNCTION_TARGET_IP} -o StrictHostKeyChecking=no "sudo rm -rf ${RESULTS_PATH}"

ssh-keygen -f "/home/ubuntu/.ssh/known_hosts" -R ${WORKLOAD_TARGET_IP}
ssh -i ${ID_RSA_PATH} ubuntu@${WORKLOAD_TARGET_IP} -o StrictHostKeyChecking=no "sudo rm -rf ${RESULTS_PATH}"

echo -e "${YELLOW}BUILDING UP${NC}"
ssh -i ${ID_RSA_PATH} ubuntu@${FUNCTION_TARGET_IP} -o StrictHostKeyChecking=no "${CD_TO_SCRIPTS_PATH}; mkdir -p ${RESULTS_PATH} ;git pull; sudo bash build.sh"
ssh -i ${ID_RSA_PATH} ubuntu@${WORKLOAD_TARGET_IP} -o StrictHostKeyChecking=no "${CD_TO_SCRIPTS_PATH}; mkdir -p ${RESULTS_PATH} ;git pull"

for expid in `seq ${INITIAL_EXPID} ${NUMBER_OF_EXPERIMENTS}`;
do
    for flag in ${FLAGS};
    do
        echo -e "${YELLOW}TEARING DOWN CONTAINERS${NC}"
        ssh -i ${ID_RSA_PATH} ubuntu@${FUNCTION_TARGET_IP} -o StrictHostKeyChecking=no "${CD_TO_SCRIPTS_PATH}; sudo bash teardown.sh"

        echo -e "${YELLOW}SETTING UP CONTAINERS${NC}"
        ssh -i ${ID_RSA_PATH} ubuntu@${WORKLOAD_TARGET_IP} -o StrictHostKeyChecking=no "${CD_TO_SCRIPTS_PATH}; sudo LOG_PATH=${RESULTS_PATH} EXPID=${expid} FUNCTION_TARGET_IP=${FUNCTION_TARGET_IP} GCI_FLAG=${flag} bash setup.sh"

        echo -e "${RED}RUNNING WORKLOAD FOR ${CONTAINER_TAG} EXPID ${EXPID}${NC}"
        ssh -i ${ID_RSA_PATH} ubuntu@${WORKLOAD_TARGET_IP} -o StrictHostKeyChecking=no "${CD_TO_SCRIPTS_PATH}; sudo NUMBER_OF_REQUESTS=${NUMBER_OF_REQUESTS} FUNCTION_TARGET_IP=${FUNCTION_TARGET_IP} TARGET_PORT=${FUNCTION_TARGET_PORT} RESULTS_PATH=${RESULTS_PATH} FILE_NAME=${RESULTS_PATH}${flag}${expid}.csv bash workload.sh"
    done;
done

mkdir -p ${LOCAL_RESULTS_PATH}

scp -i ${ID_RSA_PATH} -o StrictHostKeyChecking=no ubuntu@${FUNCTION_TARGET_IP}:"${RESULTS_PATH}*.log" $LOCAL_RESULTS_PATH
ssh -i ${ID_RSA_PATH} ubuntu@${FUNCTION_TARGET_IP} -o StrictHostKeyChecking=no "sudo rm -rf ${RESULTS_PATH}"

scp -i ${ID_RSA_PATH} -o StrictHostKeyChecking=no ubuntu@${WORKLOAD_TARGET_IP}:"${RESULTS_PATH}*.csv" $LOCAL_RESULTS_PATH
ssh -i ${ID_RSA_PATH} ubuntu@${WORKLOAD_TARGET_IP} -o StrictHostKeyChecking=no "sudo rm -rf ${RESULTS_PATH}"
