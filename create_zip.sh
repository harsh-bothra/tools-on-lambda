#!/bin/bash

rm -f lambda.zip && zip lambda.zip main subfinder
echo "Created lambda.zip successfully"