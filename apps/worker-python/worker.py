import os
import json
import time
import boto3
import redis
from botocore.exceptions import ClientError

# Configuration
SQS_QUEUE_URL = os.getenv("SQS_QUEUE_URL", "http://localhost:4566/000000000000/pixel-queue")
REDIS_ADDR = os.getenv("REDIS_ADDR", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
DYNAMODB_TABLE_NAME = os.getenv("DYNAMODB_TABLE_NAME", "PixelBoard")
AWS_ENDPOINT = os.getenv("AWS_ENDPOINT", None) # For LocalStack

# Setup Clients
session = boto3.Session()

# SQS Client
sqs = session.client('sqs', endpoint_url=AWS_ENDPOINT, region_name='ap-northeast-2')

# DynamoDB Resource
dynamodb = session.resource('dynamodb', endpoint_url=AWS_ENDPOINT, region_name='ap-northeast-2')
table = dynamodb.Table(DYNAMODB_TABLE_NAME)

# Redis Client
r = redis.Redis(host=REDIS_ADDR, port=REDIS_PORT, decode_responses=True)

def process_message(message):
    try:
        body = json.loads(message['Body'])
        x = body.get('x')
        y = body.get('y')
        color = body.get('color')
        user_id = body.get('userId')
        timestamp = body.get('timestamp')

        print(f"Processing pixel: ({x}, {y}) - {color}")

        # 1. Update DynamoDB
        table.put_item(
            Item={
                'coordinate': f"{x}:{y}",
                'color': color,
                'updatedBy': user_id,
                'timestamp': timestamp,
                'last_updated': int(time.time())
            }
        )

        # 2. Publish to Redis (Broadcast)
        # We publish the exact same message body back to Redis for the Gateway to broadcast
        r.publish('pixel-updates', json.dumps(body))

    except ClientError as e:
        print(f"DynamoDB Error: {e}")
    except Exception as e:
        print(f"Error processing message: {e}")

def ensure_sqs_queue():
    """Ensure SQS queue exists, create if not"""
    queue_name = "pixel-queue"
    try:
        sqs.get_queue_url(QueueName=queue_name)
        print(f"SQS queue '{queue_name}' already exists")
    except ClientError as e:
        if e.response['Error']['Code'] == 'AWS.SimpleQueueService.NonExistentQueue':
            print(f"Creating SQS queue: {queue_name}")
            sqs.create_queue(QueueName=queue_name)
            print(f"SQS queue created successfully")
        else:
            raise

def main():
    print("Worker started. Ensuring SQS queue exists...")
    ensure_sqs_queue()
    
    print("Polling SQS...")
    
    while True:
        try:
            response = sqs.receive_message(
                QueueUrl=SQS_QUEUE_URL,
                MaxNumberOfMessages=10,
                WaitTimeSeconds=20
            )

            messages = response.get('Messages', [])
            for msg in messages:
                process_message(msg)
                
                # Delete message after processing
                sqs.delete_message(
                    QueueUrl=SQS_QUEUE_URL,
                    ReceiptHandle=msg['ReceiptHandle']
                )

        except Exception as e:
            print(f"Polling error: {e}")
            time.sleep(5)

if __name__ == "__main__":
    main()
