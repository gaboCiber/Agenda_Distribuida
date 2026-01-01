package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ServiceRegistryClient maneja la comunicación con el servicio de registro
type ServiceRegistryClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewServiceRegistryClient crea una nueva instancia de ServiceRegistryClient
func NewServiceRegistryClient(baseURL string, logger *zap.Logger) *ServiceRegistryClient {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &ServiceRegistryClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// SetBaseURL actualiza la URL base del cliente
func (c *ServiceRegistryClient) SetBaseURL(baseURL string) {
	c.baseURL = strings.TrimSuffix(baseURL, "/")
}

func (c *ServiceRegistryClient) RegisterService(serviceName, address string) error {
	url := fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, serviceName)

	data := map[string]string{"address": address}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creando petición: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error registrando servicio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("error registrando servicio: %s", resp.Status)
	}

	c.logger.Info("Servicio registrado exitosamente",
		zap.String("service", serviceName),
		zap.String("address", address))

	return nil
}

// SendHeartbeat envía un latido para mantener el registro activo
func (c *ServiceRegistryClient) SendHeartbeat(serviceName string) error {
	url := fmt.Sprintf("%s/api/v1/services/%s/heartbeat", c.baseURL, serviceName)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("error creando petición de heartbeat: %v", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando heartbeat: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("error en heartbeat: %s", resp.Status)
	}

	return nil
}

// GetContainerIP obtiene la IP del contenedor actual
func GetContainerIP() (string, error) {
	// Primero intentamos con el nombre del host (funciona en Docker)
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Intentar resolver el hostname a una IP
		addrs, err := net.LookupIP(hostname)
		if err == nil {
			for _, addr := range addrs {
				if ipv4 := addr.To4(); ipv4 != nil {
					return ipv4.String(), nil
				}
			}
		}
	}

	// Si lo anterior falla, intentamos con las interfaces de red
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("error obteniendo interfaces de red: %v", err)
	}

	for _, iface := range ifaces {
		// Ignorar interfaces apagadas o loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Verificar que sea una dirección IPv4 válida
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			// Verificar que no sea una dirección link-local
			if !ip.IsLinkLocalUnicast() {
				return ip.String(), nil
			}
		}
	}

	// Último recurso: usar la variable de entorno HOSTNAME (común en Docker)
	if hostIP := os.Getenv("HOSTNAME"); hostIP != "" {
		return hostIP, nil
	}

	return "", fmt.Errorf("no se pudo determinar la IP del contenedor")
}
