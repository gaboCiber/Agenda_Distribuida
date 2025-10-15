# Cliente HTTP para comunicaciones con otros servicios
import httpx
from fastapi import HTTPException

class ServiceClient:
    """Cliente para hacer peticiones HTTP a otros servicios"""

    def __init__(self, base_url: str, timeout: float = 30.0):
        self.base_url = base_url
        self.timeout = timeout

    async def request(self, endpoint: str, method: str = "GET", data: dict = None, headers: dict = None):
        """Hace una petición HTTP al servicio"""
        url = f"{self.base_url}{endpoint}"

        request_headers = {"Content-Type": "application/json"}
        if headers:
            request_headers.update(headers)

        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                if method == "GET":
                    response = await client.get(url, headers=request_headers)
                elif method == "POST":
                    response = await client.post(url, json=data, headers=request_headers)
                elif method == "PUT":
                    response = await client.put(url, json=data, headers=request_headers)
                elif method == "DELETE":
                    response = await client.delete(url, headers=request_headers)
                else:
                    raise HTTPException(status_code=500, detail=f"Unsupported method: {method}")

                return response

        except httpx.ConnectError:
            raise HTTPException(status_code=503, detail=f"{self.base_url} no disponible")
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"Error interno: {str(e)}")

# Instancias de clientes para cada servicio
events_service_client = ServiceClient("http://agenda-events-service:8002")
groups_service_client = ServiceClient("http://agenda-groups-service:8003")
users_service_client = ServiceClient("http://agenda-users-service:8001")

# Cliente interno para el API Gateway (para llamadas recursivas)
api_gateway_client = ServiceClient("http://localhost:8000")

async def make_api_request(endpoint: str, method: str = "GET", data: dict = None):
    """Hace una petición interna al API Gateway"""
    return await api_gateway_client.request(endpoint, method, data)

async def make_events_service_request(endpoint: str, method: str = "GET", data: dict = None, headers: dict = None, params: dict = None):
    """Hace una petición al Events Service"""
    url = f"{events_service_client.base_url}{endpoint}"

    request_headers = {"Content-Type": "application/json"}
    if headers:
        request_headers.update(headers)

    try:
        async with httpx.AsyncClient(timeout=events_service_client.timeout) as client:
            if method == "GET":
                response = await client.get(url, headers=request_headers, params=params)
            elif method == "POST":
                response = await client.post(url, json=data, headers=request_headers)
            elif method == "PUT":
                response = await client.put(url, json=data, headers=request_headers)
            elif method == "DELETE":
                response = await client.delete(url, headers=request_headers)
            else:
                raise HTTPException(status_code=500, detail=f"Unsupported method: {method}")

            return response

    except httpx.ConnectError:
        raise HTTPException(status_code=503, detail=f"Events Service no disponible")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error interno: {str(e)}")

async def make_groups_service_request(endpoint: str, method: str = "GET", data: dict = None, headers: dict = None):
    """Hace una petición al Groups Service"""
    return await groups_service_client.request(endpoint, method, data, headers)

async def make_users_service_request(endpoint: str, method: str = "GET", data: dict = None, headers: dict = None):
    """Hace una petición al Users Service"""
    return await users_service_client.request(endpoint, method, data, headers)