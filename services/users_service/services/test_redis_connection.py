import asyncio
import redis
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

async def test_redis_connection():
    try:
        # Connect to Redis
        client = redis.Redis(
            host='agenda-bus-redis',
            port=6379,
            db=0,
            socket_connect_timeout=5,
            socket_timeout=5,
            decode_responses=True
        )
        
        # Test connection
        logger.info("Testing Redis connection...")
        if client.ping():
            logger.info("✅ Successfully connected to Redis")
            
            # Test pub/sub
            pubsub = client.pubsub()
            logger.info("Subscribing to 'test_channel'...")
            pubsub.subscribe('test_channel')
            logger.info("✅ Successfully subscribed to 'test_channel'")
            
            # Send a test message
            logger.info("Publishing test message...")
            client.publish('test_channel', 'test message')
            
            # Try to receive a message
            logger.info("Waiting for messages...")
            message = pubsub.get_message(timeout=5)
            if message:
                logger.info(f"✅ Received message: {message}")
            else:
                logger.warning("❌ No message received within timeout")
                
        else:
            logger.error("❌ Failed to connect to Redis")
            
    except Exception as e:
        logger.error(f"❌ Error: {e}")
    finally:
        if 'client' in locals():
            client.close()

if __name__ == "__main__":
    asyncio.run(test_redis_connection())
