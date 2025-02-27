package producer

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	config "github.com/splitio/go-split-commons/v3/conf"
	"github.com/splitio/go-split-commons/v3/dtos"
	"github.com/splitio/go-split-commons/v3/service"
	"github.com/splitio/go-split-commons/v3/storage/redis"
	"github.com/splitio/go-toolkit/v4/logging"
	"github.com/splitio/go-toolkit/v4/nethelpers"
	"github.com/splitio/split-synchronizer/v4/appcontext"
	"github.com/splitio/split-synchronizer/v4/conf"
	"github.com/splitio/split-synchronizer/v4/log"
	"github.com/splitio/split-synchronizer/v4/splitio/util"
)

func parseTLSConfig(opt conf.RedisSection) (*tls.Config, error) {
	if !opt.TLS {
		return nil, nil
	}

	if opt.SentinelReplication || opt.ClusterMode {
		return nil, errors.New("TLS encryption cannot be used with Sentinel replication or Cluster mode enabled")
	}

	cfg := tls.Config{}

	if opt.TLSServerName != "" {
		cfg.ServerName = opt.TLSServerName
	} else {
		cfg.ServerName = opt.Host
	}

	if len(opt.TLSCACertificates) > 0 {
		certPool := x509.NewCertPool()
		for _, cacert := range opt.TLSCACertificates {
			pemData, err := ioutil.ReadFile(cacert)
			if err != nil {
				log.Instance.Error(fmt.Sprintf("Failed to load Root CA certificate: %s", cacert))
				return nil, err
			}
			ok := certPool.AppendCertsFromPEM(pemData)
			if !ok {
				log.Instance.Error(fmt.Sprintf("Failed to add certificate %s to the TLS configuration", cacert))
				return nil, fmt.Errorf("Couldn't add certificate %s to redis TLS configuration", cacert)
			}
		}
		cfg.RootCAs = certPool
	}

	cfg.InsecureSkipVerify = opt.TLSSkipNameValidation

	if opt.TLSClientKey != "" && opt.TLSClientCertificate != "" {
		certPair, err := tls.LoadX509KeyPair(
			opt.TLSClientCertificate,
			opt.TLSClientKey,
		)

		if err != nil {
			log.Instance.Error("Unable to load client certificate and private key")
			return nil, err
		}

		cfg.Certificates = []tls.Certificate{certPair}
	} else if opt.TLSClientKey != opt.TLSClientCertificate {
		// If they aren't both set, and they aren't equal, it means that only one is set, which is invalid.
		return nil, errors.New("You must provide either both client certificate and client private key, or none")
	}

	return &cfg, nil
}

func parseRedisOptions() (*config.RedisConfig, error) {
	tlsCfg, err := parseTLSConfig(conf.Data.Redis)
	if err != nil {
		return nil, errors.New("Error in Redis TLS Configuration")
	}

	redisCfg := &config.RedisConfig{
		Password:     conf.Data.Redis.Pass,
		Prefix:       conf.Data.Redis.Prefix,
		Network:      conf.Data.Redis.Network,
		MaxRetries:   conf.Data.Redis.MaxRetries,
		DialTimeout:  conf.Data.Redis.DialTimeout,
		ReadTimeout:  conf.Data.Redis.ReadTimeout,
		WriteTimeout: conf.Data.Redis.WriteTimeout,
		PoolSize:     conf.Data.Redis.PoolSize,
		TLSConfig:    tlsCfg,
	}

	if conf.Data.Redis.SentinelReplication {
		redisCfg.SentinelAddresses = strings.Split(conf.Data.Redis.SentinelAddresses, ",")
		redisCfg.SentinelMaster = conf.Data.Redis.SentinelMaster
	} else if conf.Data.Redis.ClusterMode {
		redisCfg.ClusterKeyHashTag = conf.Data.Redis.ClusterKeyHashTag
		redisCfg.ClusterNodes = strings.Split(conf.Data.Redis.ClusterNodes, ",")
	} else {
		redisCfg.Host = conf.Data.Redis.Host
		redisCfg.Port = conf.Data.Redis.Port
		redisCfg.Database = conf.Data.Redis.Db
	}
	return redisCfg, nil
}

func isValidApikey(splitFetcher service.SplitFetcher) bool {
	_, err := splitFetcher.Fetch(time.Now().UnixNano()/int64(time.Millisecond), false)
	return err == nil
}

func startLoop(loopTime int64) {
	for {
		time.Sleep(time.Duration(loopTime) * time.Millisecond)
	}
}

func sanitizeRedis(miscStorage *redis.MiscStorage, logger logging.LoggerInterface) error {
	if miscStorage == nil {
		return errors.New("Could not sanitize redis")
	}
	currentHash := util.HashAPIKey(conf.Data.APIKey)
	currentHashAsStr := strconv.Itoa(int(currentHash))
	defer miscStorage.SetApikeyHash(currentHashAsStr)

	if conf.Data.Redis.ForceFreshStartup {
		logger.Warning("Fresh startup requested. Cleaning up redis before initializing.")
		miscStorage.ClearAll()
		return nil
	}

	previousHashStr, err := miscStorage.GetApikeyHash()
	if err != nil && err.Error() != redis.ErrorHashNotPresent { // Missing hash is not considered an error
		return err
	}

	if currentHashAsStr != previousHashStr {
		logger.Warning("Previous apikey is missing/different from current one. Cleaning up redis before startup.")
		miscStorage.ClearAll()
	}
	return nil
}

func getMetadata() dtos.Metadata {
	instanceName := "unknown"
	ipAddress := "unknown"
	if conf.Data.IPAddressesEnabled {
		ip, err := nethelpers.ExternalIP()
		if err == nil {
			ipAddress = ip
			instanceName = fmt.Sprintf("ip-%s", strings.Replace(ipAddress, ".", "-", -1))
		}
	}

	return dtos.Metadata{
		MachineIP:   ipAddress,
		MachineName: instanceName,
		SDKVersion:  appcontext.VersionHeader(),
	}
}
