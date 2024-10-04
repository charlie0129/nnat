# nnat (No NAT)

`nnat` is a L4 reverse proxy that proxies requests to a server behind NAT, similar to [fatedier/frp](https://github.com/fatedier/frp). 

It is just a hobby project that I happen to be curious about.

One day I had a dreaming about reverse proxying to a server behind NAT (wtf?) and when I woke up, I thought it was a good idea to implement it. So here we are.

I know there are projects that are way better than this one, but why not practice some networking skills?

## Architecture

For simplicity, we will only use one `nnatc` and one `dst` in the following examples. The actual implementation can handle multiple `nnatc` and `dst`.

### Components

![Connection Pool](docs/connpool.svg)

- `nnats`: The server that listens for incoming connections. Sits in the public network.
- `nnatc`: The client that connects to the `nnats`. Sits behind NAT.

`nnats` and `nnatc` communicate with each other using a custom protocol. A TCP connection pool is formed to handle multiple connections from the user.

### Connection Direction

![Connection Direction](docs/conndirection.svg)

- A user connects to the `nnats`.
- The `nnatc` connects to `nnats` and establishes a tunnel. Since the `nnatc` is behind NAT, it cannot be directly connected to by the user or `nnats`.
- The `nnatc` connects to the server (dst).

Note that we are only describing who initiates the connection. The data flow is always bidirectional.

### Data Flow

![Data Flow](docs/dataflow.svg)

The dataflow is as follows:

1. A `user` connects to the `nnats`, as if it was connecting to `dst` and sends some data.
2. `nnats` selects one connection from the connection pool (between `nnatc` and `nnats`) that acts a tunnel for this connection from `user`. The data is forwarded to the `nnatc`.
3. `nnatc` receives the data from `nnats` and opens a connection to `dst`. The data is forwarded to `dst`.
4. `dst` processes the data and sends the response back to `nnatc`.
5. `nnatc` receives the response from `dst` and forwards it to `nnats`.
6. `nnats` forwards the response to the `user`.

## Usage

Let's use a simple example (one `nnatc`, one `dst`). Say we have a server `dst` (behind NAT) that listens on `:8080`. We want to expose this server to the public network on `:18080` (or any other port).

![Example](docs/example.svg)

Here is a list of things to configure:

- `nnatc`
    - Identify itself (so the `nnats` can forward the right data to the right `nnatc`).
    - Connect to the `nnats`.
    - Forward the data from `nnats` to `dst`.
- `nnats`
    - Listen for incoming connections from `nnatc` on `:9253`.
    - Listen for incoming connections from the user on `:18080` and forward the data to the right `nnatc`.

### Build

```bash
$ make
```

### Generate a secret to identify the `nnatc`

A secret is used to identify the `nnatc` to the `nnats`. The `nnats` will forward the data to the right `nnatc` based on this secret.

The secret must be a 16-byte array (binary). You can use a 16-character string for simplicity.

The `nnatc` and `nnats` cli are expecting the secret to be base64 encoded. You can use the following command to generate a secret:

```console
$ echo -n "1234123412341234" | base64
MTIzNDEyMzQxMjM0MTIzNA==
```

### Run nnats

We need to let the `nnats` know which port to listen on and which `nnatc` to forward the data to. We can achieve this using `-conf` argument. Its value is formed as `<base64-secret>:<port>`. `-conf` can be specified multiple times to add multiple `nnatc`s. 

We are only adding one `nnatc` in this example (with the secret generated above `MTIzNDEyMzQxMjM0MTIzNA==` and listening on port `:18080`).

```console
$ bin/nnats \
    -listen-address :9253 \
    -conf MTIzNDEyMzQxMjM0MTIzNA==:18080
```

Note: `-listen-address` specifies the port to accept incoming connections from **`nnatc`**, while `-conf` specifies the port to accept incoming connections from the **user** and the `nnatc` instance to forward the data to.

### Run nnatc

```console
$ bin/nnatc \
    -conn-pool-size 10 \
    -connection-secret MTIzNDEyMzQxMjM0MTIzNA== \
    -destination-address 10.0.0.2:8080 \
    -server-address 1.3.3.3:9253
```

- `-conn-pool-size` specifies the number of connections to keep open between `nnatc` and `nnats` to improve performance.
- `-connection-secret` is the secret generated above that identifies this `nnatc` instance to the `nnats`.
- `-destination-address` is the address of the server to forward the data to.
- `-server-address` is the address of the `nnats`.

### Test

Now you can test the setup by connecting to `1.3.3.3:18080` and see if the data is forwarded to `dst`.

### Multiple `nnatc` instances

You can run multiple `nnatc` instances to handle multiple servers behind NAT. Just:

- Generate a secret for each `nnatc` instance.
- Add each `nnatc` instance to the `nnats` using `-conf <base64-secret>:<port>`.
- Run each `nnatc` instance with the corresponding secret and destination address.

## Internals

### Protocol

The protocol between `nnatc` and `nnats` is a simple binary protocol. 

1. Once the `nnatc` makes a connection to the `nnats`, it sends a `ClientHello` message which contains the secret to identify itself. 
2. The `nnats` responds with a `ServerHello` message which contains whether the connection is accepted or not and the port that the `nnats` will listen on for incoming connections from the user.

After the initial handshake, the data is copied unmodified between the `user` and `dst`.

### Connection Pool

Since each connection from user will consume a connection between `nnatc` and `nnats` (by design). So you will need to keep a pool of connections between `nnatc` and `nnats` to handle multiple connections from the user.

> You may ask: why not use the same connection between `nnatc` and `nnats` for all connections from the user? The answer is that multiplexing is hard (:

The connection pool is a simple implementation that keeps a list of connections between `nnatc` and `nnats` that can improve performance during traffic bursts.

When the `nnatc` is started, it opens `conn-pool-size` connections to the `nnats` and keeps them open. The `nnats` will remember the public port that listens on and the connections associated with this `nnatc` instance. When a new connection is made from the user, the `nnats` will know which `nnatc` this connection is destined to (each public port is bind to a `nnatc` isntance) and select a connection from the pool and forwards the data to the `nnatc`. Once the connection is closed, a new connection is opened by the `nnatc` and added to the pool to keep the pool size constant.
