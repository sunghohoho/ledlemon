#!/bin/bash
awslocal sqs create-queue --queue-name pixel-queue
awslocal dynamodb create-table \
    --table-name PixelBoard \
    --attribute-definitions AttributeName=coordinate,AttributeType=S \
    --key-schema AttributeName=coordinate,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

awslocal dynamodb create-table \
    --table-name ChatMessages \
    --attribute-definitions AttributeName=messageId,AttributeType=S \
    --key-schema AttributeName=messageId,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST
