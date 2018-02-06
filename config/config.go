package config

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"github.com/wvanbergen/kazoo-go"
	"gopkg.in/yaml.v2"
)

// App defines Kafka-Pixy application configuration. It mirrors the structure
// of the JSON configuration file.
type App struct {
	// TCP address that gRPC API server should listen on.
	GRPCAddr string `yaml:"grpc_addr"`

	// TCP address that HTTP API server should listen on.
	TCPAddr string `yaml:"tcp_addr"`

	// Unix domain socket address that HTTP API server should listen on.
	// Listening on a unix domain socket is disabled by default.
	UnixAddr string `yaml:"unix_addr"`

	// An arbitrary number of proxies to different Kafka/ZooKeeper clusters can
	// be configured. Each proxy configuration is identified by a cluster name.
	Proxies map[string]*Proxy `yaml:"proxies"`

	// Default cluster is the one to be used in API calls that do not start with
	// prefix `/clusters/<cluster>`. If it is not explicitly provided, then the
	// one mentioned in the `Proxies` section first is assumed.
	DefaultCluster string `yaml:"default_cluster"`
}

// Proxy defines configuration of a proxy to a particular Kafka/ZooKeeper
// cluster.
type Proxy struct {
	// Unique ID that identifies a Kafka-Pixy instance in both ZooKeeper and
	// Kafka. It is automatically generated by default and it is recommended to
	// leave it like that.
	ClientID string `yaml:"client_id"`

	Kafka struct {

		// List of seed Kafka peers that Kafka-Pixy should access to resolve
		// the Kafka cluster topology.
		SeedPeers []string `yaml:"seed_peers"`

		// Version of the Kafka cluster. Supported versions are 0.8.2.2 - 1.0.0
		Version KafkaVersion
	} `yaml:"kafka"`

	ZooKeeper struct {

		// List of seed ZooKeeper peers that Kafka-Pixy should access to
		// resolve the ZooKeeper cluster topology.
		SeedPeers []string `yaml:"seed_peers"`

		// Path to the directory where Kafka keeps its data.
		Chroot string `yaml:"chroot"`
	} `yaml:"zoo_keeper"`

	Producer struct {

		// Size of all buffered channels created by the producer module.
		ChannelBufferSize int `yaml:"channel_buffer_size"`

		// The type of compression to use on messages.
		Compression Compression `yaml:"compression"`

		// The best-effort number of bytes needed to trigger a flush.
		FlushBytes int `yaml:"flush_bytes"`

		// The best-effort frequency of flushes.
		FlushFrequency time.Duration `yaml:"flush_frequency"`

		// How long to wait for the cluster to settle between retries.
		RetryBackoff time.Duration `yaml:"retry_backoff"`

		// The total number of times to retry sending a message.
		RetryMax int `yaml:"retry_max"`

		// The level of acknowledgement reliability needed from the broker.
		RequiredAcks RequiredAcks `yaml:"required_acks"`

		// Period of time that Kafka-Pixy should keep trying to submit buffered
		// messages to Kafka. It is recommended to make it large enough to survive
		// a ZooKeeper leader election in your setup.
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"producer"`

	Consumer struct {

		// Period of time that Kafka-Pixy should wait for an acknowledgement
		// before retrying.
		AckTimeout time.Duration `yaml:"ack_timeout"`

		// Size of all buffered channels created by the consumer module.
		ChannelBufferSize int `yaml:"channel_buffer_size"`

		// The number of bytes of messages to attempt to fetch for each
		// topic-partition in each fetch request. These bytes will be read into
		// memory for each partition, so this helps control the memory used by
		// the consumer. The fetch request size must be at least as large as
		// the maximum message size the server allows or else it is possible
		// for the producer to send messages larger than the consumer can fetch.
		FetchMaxBytes int `yaml:"fetch_max_bytes"`

		// The maximum amount of time the server will block before answering
		// the fetch request if there isn't data immediately available.
		FetchMaxWait time.Duration `yaml:"fetch_max_wait"`

		// Consume request will wait at most this long for a message from a
		// topic to become available before expiring.
		LongPollingTimeout time.Duration `yaml:"long_polling_timeout"`

		// The maximum number of unacknowledged messages allowed for a
		// particular group-topic-partition at a time. When this number is
		// reached subsequent consume requests will return long polling timeout
		// errors, until some of the pending messages are acknowledged.
		MaxPendingMessages int `yaml:"max_pending_messages"`

		// The maximum number of retries Kafka-Pixy will make to offer an
		// unack message. Messages that exceeded the number of retries are
		// discarded by Kafka-Pixy and acknowledged in Kafka. Zero retries
		// means that messages will be offered just once.
		//
		// If you want Kafka-Pixy to retry indefinitely, then set this
		// parameter to -1.
		MaxRetries int `yaml:"max_retries"`

		// How frequently to commit offsets to Kafka.
		OffsetsCommitInterval time.Duration `yaml:"offsets_commit_interval"`

		// How long to wait for an offset to be committed by Kafka before
		// retrying.
		OffsetsCommitTimeout time.Duration `yaml:"offsets_commit_timeout"`

		// Kafka-Pixy should wait this long after it gets notification that a
		// consumer joined/left a consumer group it is a member of before
		// rebalancing.
		RebalanceDelay time.Duration `yaml:"rebalance_delay"`

		// If a request to a Kafka-Pixy fails for any reason, then it should
		// wait this long before retrying.
		RetryBackoff time.Duration `yaml:"retry_backoff"`

		// Period of time that Kafka-Pixy should keep subscription to
		// a topic by a group in absence of requests from the consumer group.
		SubscriptionTimeout time.Duration `yaml:"subscription_timeout"`
	} `yaml:"consumer"`
}

type KafkaVersion struct {
	v sarama.KafkaVersion
}

func (kv *KafkaVersion) UnmarshalText(text []byte) error {
	str := string(text)
	v, ok := map[string]sarama.KafkaVersion{
		"0.8.2.2":  sarama.V0_8_2_2,
		"0.9.0.0":  sarama.V0_9_0_0,
		"0.9.0.1":  sarama.V0_9_0_1,
		"0.10.0.0": sarama.V0_10_0_0,
		"0.10.0.1": sarama.V0_10_0_1,
		"0.10.1.0": sarama.V0_10_1_0,
		"0.10.2.0": sarama.V0_10_2_0,
		"0.10.2.1": sarama.V0_10_2_0, // sarama does not define V0_10_2_1
		"0.11.0.0": sarama.V0_11_0_0,
		"0.11.0.1": sarama.V0_11_0_0, // sarama does not define V0_11_0_1
		"0.11.0.2": sarama.V0_11_0_0, // sarama does not define V0_11_0_2
		"1.0.0":    sarama.V1_0_0_0,
	}[str]
	if !ok {
		return errors.Errorf("bad kafka version, %s", str)
	}
	kv.v = v
	return nil
}

func (kv *KafkaVersion) Set(v sarama.KafkaVersion) {
	kv.v = v
}

func (kv *KafkaVersion) IsAtLeast(v sarama.KafkaVersion) bool {
	return kv.v.IsAtLeast(v)
}

type Compression sarama.CompressionCodec

func (c *Compression) UnmarshalText(text []byte) error {
	str := string(text)
	v, ok := map[string]sarama.CompressionCodec{
		"none":   sarama.CompressionNone,
		"gzip":   sarama.CompressionGZIP,
		"snappy": sarama.CompressionSnappy,
		"lz4":    sarama.CompressionLZ4,
	}[str]
	if !ok {
		return errors.Errorf("bad compression, %s", str)
	}
	*c = Compression(v)
	return nil
}

type RequiredAcks sarama.RequiredAcks

func (ra *RequiredAcks) UnmarshalText(text []byte) error {
	str := string(text)
	v, ok := map[string]sarama.RequiredAcks{
		"no_response":    sarama.NoResponse,
		"wait_for_local": sarama.WaitForLocal,
		"wait_for_all":   sarama.WaitForAll,
	}[str]
	if !ok {
		return errors.Errorf("bad compression, %s", str)
	}
	*ra = RequiredAcks(v)
	return nil
}

func (p *Proxy) KazooCfg() *kazoo.Config {
	kazooCfg := kazoo.NewConfig()
	kazooCfg.Chroot = p.ZooKeeper.Chroot
	// ZooKeeper documentation says following about the session timeout: "The
	// current (ZooKeeper) implementation requires that the timeout be a
	// minimum of 2 times the tickTime (as set in the server configuration) and
	// a maximum of 20 times the tickTime". The default tickTime is 2 seconds.
	// See http://zookeeper.apache.org/doc/trunk/zookeeperProgrammers.html#ch_zkSessions
	kazooCfg.Timeout = 15 * time.Second
	return kazooCfg
}

// SaramaProducerCfg returns a config for sarama producer.
func (p *Proxy) SaramaProducerCfg() *sarama.Config {
	saramaCfg := sarama.NewConfig()
	saramaCfg.ChannelBufferSize = p.Producer.ChannelBufferSize
	saramaCfg.ClientID = p.ClientID
	saramaCfg.Version = p.Kafka.Version.v

	saramaCfg.Producer.Compression = sarama.CompressionCodec(p.Producer.Compression)
	saramaCfg.Producer.Flush.Frequency = p.Producer.FlushFrequency
	saramaCfg.Producer.Flush.Bytes = p.Producer.FlushBytes
	saramaCfg.Producer.Retry.Backoff = p.Producer.RetryBackoff
	saramaCfg.Producer.Retry.Max = p.Producer.RetryMax
	saramaCfg.Producer.RequiredAcks = sarama.RequiredAcks(p.Producer.RequiredAcks)
	return saramaCfg
}

func (p *Proxy) SaramaClientCfg() *sarama.Config {
	saramaCfg := sarama.NewConfig()
	saramaCfg.ChannelBufferSize = p.Consumer.ChannelBufferSize
	saramaCfg.ClientID = p.ClientID
	saramaCfg.Version = p.Kafka.Version.v
	return saramaCfg
}

// DefaultApp returns default application configuration where default proxy has
// the specified cluster.
func DefaultApp(cluster string) *App {
	appCfg := newApp()
	proxyCfg := DefaultProxy()
	appCfg.Proxies[cluster] = proxyCfg
	appCfg.DefaultCluster = cluster
	return appCfg
}

// DefaultCluster returns configuration used by default.
func DefaultProxy() *Proxy {
	return defaultProxyWithClientID(newClientID())
}

// FromYAML parses configuration from a YAML file and performs basic
// validation of parameters.
func FromYAMLFile(filename string) (*App, error) {
	configFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()
	data, err := ioutil.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	appCfg, err := FromYAML(data)
	if err != nil {
		return nil, err
	}
	return appCfg, nil
}

// FromYAML parses configuration from a YAML string and performs basic
// validation of parameters.
func FromYAML(data []byte) (*App, error) {
	var prob proxyProb
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, errors.Wrap(err, "failed to parse config")
	}

	appCfg := newApp()
	clientID := newClientID()

	for _, proxyItem := range prob.Proxies {
		cluster, ok := proxyItem.Key.(string)
		if !ok {
			return nil, errors.Errorf("invalid cluster, %v", cluster)
		}
		// A hack with marshaling and unmarshaled of a Proxy structure is used
		// here to preserve default values. If we try to unmarshal entire App
		// config, then proxy structures are overridden with zero Proxy values.
		encodedProxyCfg, err := yaml.Marshal(proxyItem.Value)
		if err != nil {
			panic(err)
		}
		proxyCfg := defaultProxyWithClientID(clientID)
		if err := yaml.Unmarshal(encodedProxyCfg, proxyCfg); err != nil {
			return nil, errors.Wrapf(err, "failed to parse proxy config, cluster=%s", cluster)
		}
		appCfg.Proxies[cluster] = proxyCfg
		if appCfg.DefaultCluster == "" {
			appCfg.DefaultCluster = cluster
		}
	}

	if err := appCfg.validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config parameter")
	}
	return appCfg, nil
}

func (a *App) validate() error {
	if len(a.Proxies) == 0 {
		return errors.New("at least on proxy must be configured")
	}
	for cluster, proxyCfg := range a.Proxies {
		if err := proxyCfg.validate(); err != nil {
			return errors.Wrapf(err, "invalid config, cluster=%s", cluster)
		}
	}
	return nil
}

func (p *Proxy) validate() error {
	// Validate the Producer parameters.
	switch {
	case p.Producer.ChannelBufferSize <= 0:
		return errors.New("producer.channel_buffer_size must be > 0")
	case p.Producer.FlushBytes < 0:
		return errors.New("producer.flush_bytes must be >= 0")
	case p.Producer.FlushFrequency < 0:
		return errors.New("producer.flush_frequency must be >= 0")
	case p.Producer.RetryBackoff <= 0:
		return errors.New("producer.retry_backoff must be > 0")
	case p.Producer.RetryMax <= 0:
		return errors.New("producer.retry_max must be > 0")
	case p.Producer.ShutdownTimeout < 0:
		return errors.New("producer.shutdown_timeout must be >= 0")
	}
	// Validate the Consumer parameters.
	switch {
	case p.Consumer.AckTimeout <= 0:
		return errors.New("consumer.ack_timeout must be > 0")
	case p.Consumer.ChannelBufferSize <= 0:
		return errors.New("consumer.channel_buffer_size must be > 0")
	case p.Consumer.FetchMaxBytes <= 0:
		return errors.New("consumer.fetch_bytes must be > 0")
	case p.Consumer.LongPollingTimeout <= 0:
		return errors.New("consumer.long_polling_timeout must be > 0")
	case p.Consumer.MaxPendingMessages <= 0:
		return errors.New("consumer.max_pending_messages must be > 0")
	case p.Consumer.MaxRetries < -1:
		return errors.New("consumer.max_retries must be >= -1")
	case p.Consumer.OffsetsCommitInterval <= 0:
		return errors.New("consumer.offsets_commit_interval must be > 0")
	case p.Consumer.OffsetsCommitTimeout <= 0:
		return errors.New("consumer.offsets_commit_timeout must be > 0")
	case p.Consumer.SubscriptionTimeout <= 0:
		return errors.New("consumer.subscription_timeout must be > 0")
	case p.Consumer.RetryBackoff <= 0:
		return errors.New("consumer.retry_backoff must be > 0")
	}
	return nil
}

func newApp() *App {
	appCfg := &App{}
	appCfg.GRPCAddr = "0.0.0.0:19091"
	appCfg.TCPAddr = "0.0.0.0:19092"
	appCfg.Proxies = make(map[string]*Proxy)
	return appCfg
}

func defaultProxyWithClientID(clientID string) *Proxy {
	c := &Proxy{}
	c.ClientID = clientID
	c.ZooKeeper.SeedPeers = []string{"localhost:2181"}

	c.Kafka.SeedPeers = []string{"localhost:9092"}

	c.Kafka.Version.v = sarama.V0_8_2_2
	// If a valid Kafka version provided in an environment variable then use it
	// as the default value. This logic is only needed in tests.
	envKafkaVersion := os.Getenv("KAFKA_VERSION")
	var kv KafkaVersion
	if err := kv.UnmarshalText([]byte(envKafkaVersion)); err == nil {
		c.Kafka.Version = kv
	}

	c.Producer.ChannelBufferSize = 4096
	c.Producer.Compression = Compression(sarama.CompressionSnappy)
	c.Producer.FlushFrequency = 500 * time.Millisecond
	c.Producer.FlushBytes = 1024 * 1024
	c.Producer.RequiredAcks = RequiredAcks(sarama.WaitForAll)
	c.Producer.RetryBackoff = 10 * time.Second
	c.Producer.RetryMax = 6
	c.Producer.ShutdownTimeout = 30 * time.Second

	c.Consumer.AckTimeout = 300 * time.Second
	c.Consumer.ChannelBufferSize = 64
	c.Consumer.FetchMaxBytes = 1024 * 1024
	c.Consumer.FetchMaxWait = 250 * time.Millisecond
	c.Consumer.LongPollingTimeout = 3 * time.Second
	c.Consumer.MaxPendingMessages = 300
	c.Consumer.MaxRetries = -1
	c.Consumer.OffsetsCommitInterval = 500 * time.Millisecond
	c.Consumer.OffsetsCommitTimeout = 1500 * time.Millisecond
	c.Consumer.SubscriptionTimeout = 15 * time.Second
	c.Consumer.RetryBackoff = 500 * time.Millisecond
	return c
}

// newClientID creates a unique id that identifies this particular Kafka-Pixy
// in both Kafka and ZooKeeper.
func newClientID() string {
	hostname, err := os.Hostname()
	if err != nil {
		ip, err := getIP()
		if err != nil {
			buffer := make([]byte, 8)
			_, _ = rand.Read(buffer)
			hostname = fmt.Sprintf("%X", buffer)

		} else {
			hostname = ip.String()
		}
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	// sarama validation regexp for the client ID doesn't allow ':' characters
	timestamp = strings.Replace(timestamp, ":", ".", -1)
	return fmt.Sprintf("pixy_%s_%s_%d", hostname, timestamp, os.Getpid())
}

func getIP() (net.IP, error) {
	interfaceAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var ipv6 net.IP
	for _, interfaceAddr := range interfaceAddrs {
		if ipAddr, ok := interfaceAddr.(*net.IPNet); ok && !ipAddr.IP.IsLoopback() {
			ipv4 := ipAddr.IP.To4()
			if ipv4 != nil {
				return ipv4, nil
			}
			ipv6 = ipAddr.IP
		}
	}
	if ipv6 != nil {
		return ipv6, nil
	}
	return nil, errors.New("Unknown IP address")
}

type proxyProb struct {
	Proxies yaml.MapSlice
}
