#!/bin/bash

rm -f lambda.zip && zip lambda.zip main $1
echo "Created lambda.zip successfully"