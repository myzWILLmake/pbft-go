#!/bin/bash
if [ $1 = "start" ]; then
    for (( i=0; i<4; i++ )) do
        ./main server $i --tag=pbft-server &
    done
    for (( i=0; i<3; i++ )) do
        ./main client $i --tag=pbft-client &
    done
elif [ $1 = "kill" ]; then
    pgrep -f pbft-server | xargs kill
    pgrep -f pbft-client | xargs kill
elif [ $1 = "build" ]; then
    go build
fi