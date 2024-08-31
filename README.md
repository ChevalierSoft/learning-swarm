# learning swarm

## setup swarm cluster

Main master node :

```bash
docker swarm init --advertise-addr IP
```

Master node :

```bash
docker swarm join-token worker
```

Add Manager nodes via Master node :

```bash
docker swarm join-token worker
```

Add Worker nodes via Master node :

```bash
docker swarm join-token worker
```

## run sigle container

```bash
docker service create --name NAME -p 80:80 nginx
```

## run stack

Using docker compose

```bash
docker stack deploy -c docker-compose.yaml stack-name
```
