#!/bin/bash
awslocal sqs create-queue --queue-name pixel-queue
awslocal dynamodb create-table \
    --table-name PixelBoard \
    --attribute-definitions AttributeName=coordinate,AttributeType=S \
    --key-schema AttributeName=coordinate,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5
