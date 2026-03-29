#!/bin/bash
# Run after kafka is ready:
# docker exec forge-kafka bash /opt/bitnami/kafka/bin/kafka-topics.sh ...

KAFKA_BIN="/opt/bitnami/kafka/bin"
BOOTSTRAP="localhost:9092"

$KAFKA_BIN/kafka-topics.sh --create --bootstrap-server $BOOTSTRAP --topic forge.task.dispatch --partitions 6 --replication-factor 1 --if-not-exists
$KAFKA_BIN/kafka-topics.sh --create --bootstrap-server $BOOTSTRAP --topic forge.task.result --partitions 6 --replication-factor 1 --if-not-exists
$KAFKA_BIN/kafka-topics.sh --create --bootstrap-server $BOOTSTRAP --topic forge.task.event --partitions 3 --replication-factor 1 --if-not-exists

echo "Kafka topics created."
