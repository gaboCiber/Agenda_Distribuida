let token = null;
let userId = null;
let currentDate = new Date();

// Load session from localStorage on page load
function loadSession() {
    console.log('🚀 [DEBUG] loadSession called');

    const savedToken = localStorage.getItem('agenda_token');
    const savedUserId = localStorage.getItem('agenda_userId');
    const savedEmail = localStorage.getItem('agenda_email');

    console.log('🔍 [DEBUG] Loading session from localStorage:', {
        token: savedToken ? 'SET' : 'MISSING',
        userId: savedUserId || 'MISSING',
        email: savedEmail || 'MISSING'
    });

    if (savedToken && savedUserId) {
        // ✅ ESTABLECER VARIABLES GLOBALES PRIMERO
        token = savedToken;
        userId = savedUserId;

        console.log('✅ Session loaded, global variables set:', { token: !!token, userId });

        // Update UI
        document.getElementById('auth-section').style.display = 'none';
        document.getElementById('dashboard').style.display = 'block';
        document.getElementById('user-info').style.display = 'flex';
        document.getElementById('user-email').textContent = savedEmail || '';

        // ✅ CARGAR DATOS SOLO SI userId ES VÁLIDO
        if (userId && userId !== 'undefined') {
            console.log('🔄 Loading user data from session...');
            // ✅ ESPERAR A QUE SE CARGUEN LOS DATOS ANTES DE RENDERIZAR
            (async () => {
                await loadEvents();
                await loadGroups();
                // renderCalendar() ya se llama dentro de loadEvents()
            })();
        } else {
            console.error('❌ Invalid userId in session:', userId);
        }
    } else {
        console.log('ℹ️ [DEBUG] No valid session found in localStorage');
    }

    console.log('🏁 [DEBUG] loadSession COMPLETED');
}

// Save session to localStorage
function saveSession(tokenValue, userIdValue, emailValue) {
    localStorage.setItem('agenda_token', tokenValue);
    localStorage.setItem('agenda_userId', userIdValue);
    localStorage.setItem('agenda_email', emailValue || '');
    
    console.log('💾 Session saved to localStorage:', {
        token: tokenValue ? 'SET' : 'MISSING',
        userId: userIdValue,
        email: emailValue
    });
}

// Clear session from localStorage
function clearSession() {
    localStorage.removeItem('agenda_token');
    localStorage.removeItem('agenda_userId');
    localStorage.removeItem('agenda_email');
    console.log('🧹 Session cleared from localStorage');
}

// ✅ CORREGIR COMPLETAMENTE apiRequest
async function apiRequest(endpoint, method = 'GET', body = null) {
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    // ✅ CONSTRUIR URL CORRECTAMENTE CON user_id
    let url = endpoint;

    // Solo agregar user_id para endpoints específicos que lo necesitan
    const needsUserId = ['/events', '/groups', '/auth/account'].some(path => endpoint.includes(path));

    if (userId && needsUserId && (method.toUpperCase() === 'GET' || method.toUpperCase() === 'DELETE')) {
        const separator = endpoint.includes('?') ? '&' : '?';
        url = `${endpoint}${separator}user_id=${encodeURIComponent(userId)}`;

        console.log(`🔧 URL construida: ${url}`);
    }

    console.log(`🌐 API Request: ${method} ${url}`, {
        hasToken: !!token,
        hasUserId: !!userId,
        userId: userId,
        originalEndpoint: endpoint,
        finalUrl: url
    });

    try {
        const response = await fetch(`/api${url}`, {
            method,
            headers,
            body: body ? JSON.stringify(body) : null
        });

        if (!response.ok) {
            const errorText = await response.text();
            console.error(`❌ API Error ${response.status}:`, errorText);
            throw new Error(errorText || `HTTP error! status: ${response.status}`);
        }

        return response.json();
    } catch (error) {
        console.error(`💥 Fetch error for ${method} ${url}:`, error);
        throw error;
    }
}

// Notification system
function showNotification(message, type = 'success') {
    const notification = document.getElementById('notification');
    const messageEl = document.getElementById('notification-message');

    notification.className = `notification ${type}`;
    messageEl.textContent = message;
    notification.style.display = 'block';

    setTimeout(() => {
        notification.style.display = 'none';
    }, 5000);
}

// Authentication functions
async function register(event) {
    event.preventDefault();

    const username = document.getElementById('reg-username').value;
    const email = document.getElementById('reg-email').value;
    const password = document.getElementById('reg-password').value;

    try {
        console.log('📝 Attempting registration for:', email);
        const result = await apiRequest('/auth/register', 'POST', { username, email, password });
        showNotification('Usuario registrado exitosamente!', 'success');
        showTab('login');
        document.getElementById('reg-username').value = '';
        document.getElementById('reg-email').value = '';
        document.getElementById('reg-password').value = '';
    } catch (error) {
        console.error('❌ Registration failed:', error);
        showNotification('Error en el registro: ' + error.message, 'error');
    }
}

// ✅ CORREGIR login - Asegurar que userId esté disponible ANTES de cargar datos
async function login(event) {
    event.preventDefault();

    const email = document.getElementById('login-email').value;
    const password = document.getElementById('login-password').value;

    try {
        console.log('🔐 Attempting login for:', email);
        
        const result = await apiRequest('/auth/login', 'POST', { email, password });
        
        // ✅ ESTABLECER userId ANTES de cualquier otra cosa
        token = result.token;
        userId = result.user_id;

        console.log('✅ Login successful, user ID set to:', userId);

        // ✅ GUARDAR EN LOCALSTORAGE INMEDIATAMENTE
        saveSession(token, userId, email);

        // ✅ ACTUALIZAR UI
        document.getElementById('auth-section').style.display = 'none';
        document.getElementById('dashboard').style.display = 'block';
        document.getElementById('user-info').style.display = 'flex';
        document.getElementById('user-email').textContent = email;

        showNotification('Sesión iniciada exitosamente!', 'success');

        // ✅ VERIFICACIÓN EXPLÍCITA ANTES DE CARGAR DATOS
        console.log('🔄 Verifying userId before loading data:', userId);
        
        if (userId && userId !== 'undefined') {
            console.log('✅ [DEBUG] userId is valid, loading data...', { userId });
            console.log('📅 [DEBUG] About to call loadEvents');
            await loadEvents();
            console.log('👥 [DEBUG] About to call loadGroups');
            await loadGroups();
            console.log('📊 [DEBUG] About to call renderCalendar');
            renderCalendar();
            console.log('✅ [DEBUG] All data loading completed');
        } else {
            console.error('❌ [DEBUG] userId is invalid:', userId);
            showNotification('Error: No se pudo obtener el ID de usuario', 'error');
        }

        // Clear form
        document.getElementById('login-email').value = '';
        document.getElementById('login-password').value = '';

    } catch (error) {
        console.error('❌ Login failed:', error);
        
        // ✅ MEJOR MANEJO DE ERRORES EN LOGIN
        let errorMessage = 'Error al iniciar sesión';
        try {
            const errorData = JSON.parse(error.message);
            errorMessage = errorData.error || errorMessage;
        } catch (e) {
            if (error.message.includes('Invalid email or password') || error.message.includes('credenciales')) {
                errorMessage = 'Email o contraseña incorrectos';
            } else if (error.message.includes('timeout')) {
                errorMessage = 'Servicio no disponible. Intente nuevamente.';
            } else {
                errorMessage = error.message;
            }
        }
        showNotification(errorMessage, 'error');
    }
}

function logout() {
    console.log('🚪 Logging out user:', userId);
    token = null;
    userId = null;

    // Clear session from localStorage
    clearSession();

    document.getElementById('auth-section').style.display = 'block';
    document.getElementById('dashboard').style.display = 'none';
    document.getElementById('user-info').style.display = 'none';

    showNotification('Sesión cerrada', 'success');
}

// Tab switching
function showTab(tab) {
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.auth-form').forEach(form => form.style.display = 'none');

    document.querySelector(`[onclick="showTab('${tab}')"]`).classList.add('active');
    document.getElementById(`${tab}-form`).style.display = 'block';
}

// Modal functions
function showModal(modalId) {
    document.getElementById(modalId).style.display = 'flex';
}

function closeModal(modalId) {
    document.getElementById(modalId).style.display = 'none';
}

// Event functions
// ✅ VERSIÓN DE EMERGENCIA - FORZAR user_id MANUALMENTE
async function loadEvents() {
    try {
        console.log('🎯 loadEvents called', { userId });

        if (!userId) {
            console.error('❌ No user ID available for loadEvents');
            showNotification('No se pudo cargar eventos: usuario no identificado', 'error');
            return;
        }

        console.log('🔍 Loading events for user:', userId);

        // ✅ FORZAR user_id MANUALMENTE EN LA URL
        const result = await apiRequest(`/events?user_id=${userId}`);
        const container = document.getElementById('events-list');
        container.innerHTML = '';

        console.log('📦 Events response:', result);

        if (result.events && result.events.length > 0) {
            console.log(`✅ Found ${result.events.length} events`);
            result.events.forEach(event => {
                const eventCard = document.createElement('div');
                eventCard.className = 'item-card';

                // ✅ CORREGIR PARSING DE FECHAS - Las fechas vienen como strings ISO
                const startTime = event.start_time ? new Date(event.start_time) : new Date();
                const endTime = event.end_time ? new Date(event.end_time) : new Date();

                console.log('📅 Event dates:', {
                    title: event.title,
                    start_time: event.start_time,
                    end_time: event.end_time,
                    parsedStart: startTime,
                    parsedEnd: endTime
                });

                eventCard.innerHTML = `
                    <div style="display: flex; justify-content: space-between; align-items: flex-start;">
                        <div style="flex: 1;">
                            <h4>${event.title || 'Sin título'}</h4>
                            <p>${event.description || 'Sin descripción'}</p>
                            <div class="date">Inicio: ${startTime.toLocaleString()}</div>
                            <div class="date">Fin: ${endTime.toLocaleString()}</div>
                            ${event.location ? `<div class="date">Ubicación: ${event.location}</div>` : ''}
                        </div>
                        <button onclick="deleteEvent('${event.id}')" class="btn-danger" style="margin-left: 10px; padding: 5px 10px; font-size: 12px;" title="Eliminar evento">🗑️</button>
                    </div>
                `;
                container.appendChild(eventCard);

                // ✅ AGREGAR EVENTO AL CALENDARIO
                addEventToCalendar(event);
            });
        } else {
            console.log('ℹ️ No events found');
            container.innerHTML = '<p>No hay eventos para mostrar</p>';
        }

        // ✅ RENDERIZAR CALENDARIO DESPUÉS DE CARGAR EVENTOS
        renderCalendar();
    } catch (error) {
        console.error('❌ Failed to load events:', error);
        showNotification('Error al cargar eventos: ' + error.message, 'error');
    }
}

// Modificar createEventFromForm para mejor manejo de errores
function createEventFromForm() {
    console.log('🎯 createEventFromForm called');

    try {
        if (!userId) {
            showNotification('Debe iniciar sesión para crear eventos', 'error');
            return;
        }

        const title = document.getElementById('event-title').value;
        const description = document.getElementById('event-description').value;
        const startTime = document.getElementById('event-start').value;
        const endTime = document.getElementById('event-end').value;
        const groupId = document.getElementById('event-group').value;
        const location = document.getElementById('event-location')?.value || '';

        console.log('📝 Form values:', { title, description, startTime, endTime, groupId, location, userId });

        // Validate required fields
        if (!title || !startTime || !endTime) {
            showNotification('Por favor complete todos los campos requeridos', 'error');
            return;
        }

        // Validate dates
        const startDate = new Date(startTime);
        const endDate = new Date(endTime);

        if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) {
            showNotification('Fechas inválidas', 'error');
            return;
        }

        if (startDate >= endDate) {
            showNotification('La fecha de fin debe ser posterior a la fecha de inicio', 'error');
            return;
        }

        // Make the API call
        const requestData = {
            title,
            description,
            start_time: startDate.toISOString(),
            end_time: endDate.toISOString(),
            user_id: userId, // ✅ INCLUIR user_id AUTOMÁTICAMENTE
            group_id: groupId || undefined,
            location: location || ''
        };

        console.log('📤 Sending request:', requestData);

        fetch('/api/events', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...(token ? { 'Authorization': `Bearer ${token}` } : {})
            },
            body: JSON.stringify(requestData)
        })
        .then(response => {
            console.log('📨 Response status:', response.status);
            if (!response.ok) {
                return response.text().then(text => {
                    throw new Error(text || `HTTP error! status: ${response.status}`);
                });
            }
            return response.json();
        })
        .then(result => {
            console.log('✅ Success response:', result);
            showNotification('Evento creado exitosamente!', 'success');
            closeModal('event-modal');

            // Clear form
            document.getElementById('event-title').value = '';
            document.getElementById('event-description').value = '';
            document.getElementById('event-start').value = '';
            document.getElementById('event-end').value = '';
            if (document.getElementById('event-location')) {
                document.getElementById('event-location').value = '';
            }

            // Recargar eventos (renderCalendar ya se llama dentro de loadEvents)
            loadEvents();
        })
        .catch(error => {
            console.error('❌ Error creating event:', error);
            // Mejor manejo de errores
            let errorMessage = 'Error al crear evento';
            try {
                const errorData = JSON.parse(error.message);
                errorMessage = errorData.error || errorMessage;
            } catch (e) {
                if (error.message.includes('Time conflict')) {
                    errorMessage = 'Ya existe un evento en ese horario. Por favor elija otro horario.';
                } else {
                    errorMessage = error.message;
                }
            }
            showNotification(errorMessage, 'error');
        });

    } catch (error) {
        console.error('💥 JavaScript error in createEventFromForm:', error);
        showNotification('Error de JavaScript: ' + error.message, 'error');
    }
}

// ✅ VERSIÓN COMPLETA CON DETERMINACIÓN DE ROLES Y COLORES
async function loadGroups() {
    try {
        console.log('🎯 [DEBUG] loadGroups called - FULL VERSION with role determination', { userId, token: !!token });

        if (!userId) {
            console.error('❌ [DEBUG] No user ID available for loadGroups');
            showNotification('No se pudo cargar grupos: usuario no identificado', 'error');
            return;
        }

        console.log('🔍 [DEBUG] Loading groups for user:', userId);

        // ✅ FORZAR user_id MANUALMENTE EN LA URL
        console.log('🌐 [DEBUG] About to call apiRequest for groups');
        const result = await apiRequest(`/groups?user_id=${userId}`);
        console.log('📦 [DEBUG] Groups response received:', result);

        const container = document.getElementById('groups-list');
        const groupSelect = document.getElementById('event-group');

        console.log('🧹 [DEBUG] Clearing containers');
        container.innerHTML = '';
        groupSelect.innerHTML = '<option value="">Sin grupo</option>';

        console.log('📦 [DEBUG] Groups response:', result);

        if (result.groups && result.groups.length > 0) {
            console.log(`✅ [DEBUG] Found ${result.groups.length} groups`);

            // ✅ DEBUG: Mostrar estructura completa de grupos con roles
            result.groups.forEach((group, index) => {
                console.log(`Group ${index}:`, {
                    id: group.id,
                    name: group.name,
                    user_role: group.user_role,  // ✅ NUEVO CAMPO DE ROL
                    creator_id: group.creator_id,
                    is_hierarchical: group.is_hierarchical,
                    all_fields: Object.keys(group) // Mostrar todos los campos disponibles
                });
            });

            // ✅ NUEVA LÓGICA: Usar rol real que viene de la API
            const processedGroups = result.groups.map((group) => {
                console.log(`🔍 [DEBUG] Processing group ${group.id} (${group.name})`);
                console.log(`🔍 [DEBUG] Group data from API:`, {
                    id: group.id,
                    name: group.name,
                    user_role: group.user_role,  // ✅ ROL REAL DEL USUARIO
                    is_hierarchical: group.is_hierarchical,
                    creator_id: group.creator_id
                });

                // ✅ USAR EL ROL QUE VIENE DIRECTAMENTE DE LA API
                const userRole = group.user_role || 'member'; // Por defecto 'member' si no viene

                console.log(`👤 [DEBUG] User role from API for group ${group.name}: ${userRole}`);

                const colorClass = getGroupColorClass(userRole, group.is_hierarchical);

                console.log(`🎨 [DEBUG] Group ${group.name}: API_role=${userRole}, hierarchical=${group.is_hierarchical}, colorClass=${colorClass}`);

                return {
                    group,
                    userRole,
                    colorClass
                };
            });

            console.log('✅ [DEBUG] All groups processed with simplified logic');

            // Ahora renderizar todas las tarjetas
            processedGroups.forEach(({ group, userRole, colorClass }) => {
                console.log(`🎨 [DEBUG] Rendering group ${group.name} with class: ${colorClass}`);

                // Add to list con color según rol
                const groupCard = document.createElement('div');
                groupCard.className = `item-card ${colorClass}`;
                groupCard.innerHTML = `
                    <h4>${group.name || 'Sin nombre'}</h4>
                    <p>${group.description || 'Sin descripción'}</p>
                    <p>Tipo: ${group.is_hierarchical ? 'Jerárquico' : 'No jerárquico'}</p>
                    <p>Rol: ${getRoleDisplayName(userRole)}</p>
                    <div class="group-actions">
                        <button onclick="showGroupMembers('${group.id}', '${group.name}')" class="btn-secondary">
                            Ver Miembros
                        </button>
                        ${userRole === 'admin' ?
                            `<button onclick="manageGroup('${group.id}')" class="btn-secondary">
                                Gestionar Grupo
                            </button>` : ''
                        }
                    </div>
                `;
                container.appendChild(groupCard);

                // Add to select
                const option = document.createElement('option');
                option.value = group.id;
                option.textContent = group.name;
                groupSelect.appendChild(option);
            });

            // Agregar función de debug después de renderizar
            setTimeout(() => {
                debugGroupColors();
                console.log('🎨 [DEBUG] Color debugging completed');
            }, 1000);

            console.log('✅ [DEBUG] Groups rendered successfully with colors');

        } else {
            console.log('ℹ️ [DEBUG] No groups found');
            container.innerHTML = '<p>No hay grupos para mostrar</p>';
        }

        console.log('🏁 [DEBUG] loadGroups COMPLETED');
    } catch (error) {
        console.error('❌ [DEBUG] Failed to load groups:', error);
        showNotification('Error al cargar grupos: ' + error.message, 'error');
    }
}

// Función para obtener el rol del usuario en un grupo específico
async function getUserRoleInGroup(groupId, userId) {
    try {
        console.log(`🔍 Checking role for user ${userId} in group ${groupId}`);

        // ✅ USAR EL ENDPOINT CORRECTO CON QUERY PARAMETER
        const result = await apiRequest(`/groups/members?group_id=${groupId}`);

        console.log(`📦 Members response for group ${groupId}:`, result);

        if (result.members && result.members.length > 0) {
            console.log(`✅ Found ${result.members.length} members in group ${groupId}`);

            // Buscar el usuario en los miembros
            const userMember = result.members.find(member => {
                // El campo puede ser user_id, userId, id, etc.
                return member.user_id === userId ||
                       member.userId === userId ||
                       member.id === userId;
            });

            if (userMember) {
                const role = userMember.role || userMember.Role || 'member';
                console.log(`✅ User role in group ${groupId}: ${role}`);
                return role;
            } else {
                console.log(`ℹ️ User ${userId} not found in group ${groupId} members`);
            }
        } else {
            console.log(`ℹ️ No members found in group ${groupId}`);
        }

        console.log(`ℹ️ User not found in group ${groupId} members, returning 'non_member'`);
        return 'non_member';
    } catch (error) {
        console.error(`❌ Failed to get user role in group ${groupId}:`, error);
        return 'unknown';
    }
}

// Función de debug para colores de grupos
function debugGroupColors() {
    const groupCards = document.querySelectorAll('#groups-list .item-card');
    console.log(`🐛 Found ${groupCards.length} group cards`);

    groupCards.forEach((card, index) => {
        const computedStyle = window.getComputedStyle(card);
        console.log(`Card ${index}:`, {
            className: card.className,
            borderLeftColor: computedStyle.borderLeftColor,
            backgroundColor: computedStyle.backgroundColor,
            innerHTML: card.innerHTML.substring(0, 100) + '...'
        });
    });
}

// Función para determinar la clase de color según el rol (SOLO 3 CASOS)
function getGroupColorClass(userRole, isHierarchical) {
    console.log(`🎨 Getting color for role: ${userRole}, hierarchical: ${isHierarchical}`);

    // SOLO 3 CASOS SEGÚN LAS INSTRUCCIONES:
    if (userRole === 'admin' && isHierarchical) {
        console.log('🔴 Admin de grupo jerárquico - ROJO');
        return 'group-admin-hierarchical';
    } else if (userRole === 'member' && isHierarchical) {
        console.log('🟢 Miembro de grupo jerárquico - VERDE');
        return 'group-member-hierarchical';
    } else if (!isHierarchical) {
        console.log('🔵 Pertenece a grupo no jerárquico - AZUL');
        return 'group-non-hierarchical';
    } else {
        console.log('⚪ Caso no definido - GRIS');
        return 'group-other';
    }
}

// Función para mostrar nombre del rol
function getRoleDisplayName(role) {
    const roleNames = {
        'admin': 'Administrador',
        'member': 'Miembro',
        'viewer': 'Visualizador',
        'non_member': 'No miembro',
        'unknown': 'Desconocido'
    };
    return roleNames[role] || role;
}

// Función para mostrar miembros del grupo
async function showGroupMembers(groupId, groupName) {
    try {
        console.log(`👥 Loading members for group ${groupId}`);

        const result = await apiRequest(`/groups/members?group_id=${groupId}`);

        // Crear modal para mostrar miembros
        const modalId = 'group-members-modal';
        if (!document.getElementById(modalId)) {
            createMembersModal(modalId);
        }

        const modal = document.getElementById(modalId);
        const membersList = document.getElementById('group-members-list');
        const modalTitle = document.getElementById('group-members-title');

        modalTitle.textContent = `Miembros de: ${groupName}`;
        membersList.innerHTML = '';

        if (result.members && result.members.length > 0) {
            console.log(`✅ Found ${result.members.length} members`);

            result.members.forEach((member, index) => {
                console.log(`👤 [DEBUG] Member ${index}:`, member); // DEBUG: Ver qué campos tiene el member

                const memberItem = document.createElement('div');
                memberItem.className = 'member-item';
                memberItem.innerHTML = `
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <strong>Usuario:</strong> ${member.username || member.Username || member.user_id || member.userId || 'Usuario desconocido'}<br>
                            <strong>Rol:</strong> ${getRoleDisplayName(member.role || member.Role)}<br>
                            <strong>Agregado:</strong> ${new Date(member.joined_at || member.JoinedAt).toLocaleDateString()}
                        </div>
                        <div class="role-badge ${member.role || member.Role}">
                            ${getRoleDisplayName(member.role || member.Role)}
                        </div>
                    </div>
                `;
                membersList.appendChild(memberItem);
            });
        } else {
            membersList.innerHTML = '<p>No hay miembros en este grupo</p>';
        }

        // Actualizar título del modal con el nombre del grupo
        const titleElement = document.getElementById('group-members-title');
        if (titleElement) {
            titleElement.textContent = `Miembros de ${groupName}`;
        }

        showModal(modalId);
    } catch (error) {
        console.error('❌ Failed to load group members:', error);
        showNotification('Error al cargar miembros del grupo: ' + error.message, 'error');
    }
}

// Función para crear modal de miembros
function createMembersModal(modalId) {
    const modalHTML = `
        <div id="${modalId}" class="modal" style="display:none;">
            <div class="modal-content">
                <div class="modal-header">
                    <h3 id="group-members-title">Miembros del Grupo</h3>
                    <span class="close" onclick="closeModal('${modalId}')">&times;</span>
                </div>
                <div style="padding: 20px;">
                    <div id="group-members-list"></div>
                </div>
            </div>
        </div>
    `;

    document.body.insertAdjacentHTML('beforeend', modalHTML);
}

// Función para gestionar grupo (solo para admins)
function manageGroup(groupId) {
    showNotification('Funcionalidad de gestión de grupos en desarrollo', 'info');
    // Aquí puedes implementar la lógica para gestionar el grupo
    console.log(`⚙️ Managing group ${groupId}`);
}

// Agregar estas funciones al objeto global window
window.showGroupMembers = showGroupMembers;
window.manageGroup = manageGroup;

async function createGroup(event) {
    event.preventDefault();

    const name = document.getElementById('group-name').value;
    const description = document.getElementById('group-description').value;
    const isHierarchical = document.getElementById('group-hierarchical').checked;

    try {
        if (!userId) {
            showNotification('Debe iniciar sesión para crear grupos', 'error');
            return;
        }

        const result = await apiRequest('/groups', 'POST', {
            name,
            description,
            user_id: userId,
            is_hierarchical: isHierarchical
        });

        showNotification('Grupo creado exitosamente!', 'success');
        closeModal('group-modal');

        // Clear form
        document.getElementById('group-name').value = '';
        document.getElementById('group-description').value = '';
        document.getElementById('group-hierarchical').checked = false;

        loadGroups();

    } catch (error) {
        console.error('❌ Failed to create group:', error);
        showNotification('Error al crear grupo: ' + error.message, 'error');
    }
}

// ✅ ALMACÉN GLOBAL DE EVENTOS PARA EL CALENDARIO
let calendarEvents = [];

// ✅ FUNCIÓN PARA AGREGAR EVENTOS AL CALENDARIO
function addEventToCalendar(event) {
    console.log('📅 Adding event to calendar:', event.title);

    // Parsear fechas correctamente
    const startDate = new Date(event.start_time);
    const endDate = new Date(event.end_time);

    // Crear entrada del evento para el calendario
    const calendarEvent = {
        id: event.id,
        title: event.title,
        description: event.description,
        startDate: startDate,
        endDate: endDate,
        location: event.location
    };

    calendarEvents.push(calendarEvent);
    console.log('✅ Event added to calendar, total events:', calendarEvents.length);
}

// Calendar functions
function renderCalendar() {
    const year = currentDate.getFullYear();
    const month = currentDate.getMonth();

    const firstDay = new Date(year, month, 1);
    const lastDay = new Date(year, month + 1, 0);
    const startDate = new Date(firstDay);
    startDate.setDate(startDate.getDate() - firstDay.getDay());

    const title = currentDate.toLocaleDateString('es-ES', { month: 'long', year: 'numeric' });
    document.getElementById('calendar-title').textContent = title;

    const grid = document.getElementById('calendar-grid');
    grid.innerHTML = '';

    // Day names
    const dayNames = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'];
    dayNames.forEach(day => {
        const dayName = document.createElement('div');
        dayName.className = 'calendar-day calendar-day-name';
        dayName.textContent = day;
        grid.appendChild(dayName);
    });

    // Calendar days
    const currentDateObj = new Date();
    for (let i = 0; i < 42; i++) {
        const dayDiv = document.createElement('div');
        dayDiv.className = 'calendar-day';

        const dayDate = new Date(startDate);
        dayDate.setDate(startDate.getDate() + i);

        const dayNumber = dayDate.getDate();
        const isCurrentMonth = dayDate.getMonth() === month;
        const isToday = dayDate.toDateString() === currentDateObj.toDateString();

        if (!isCurrentMonth) {
            dayDiv.classList.add('other-month');
        }
        if (isToday) {
            dayDiv.classList.add('today');
        }

        // ✅ BUSCAR EVENTOS PARA ESTE DÍA
        const dayEvents = calendarEvents.filter(event => {
            const eventStart = new Date(event.startDate);
            const eventEnd = new Date(event.endDate);

            // Normalizar fechas para comparación (solo fecha, sin hora)
            const dayStart = new Date(dayDate.getFullYear(), dayDate.getMonth(), dayDate.getDate());
            const dayEnd = new Date(dayDate.getFullYear(), dayDate.getMonth(), dayDate.getDate(), 23, 59, 59);

            // El evento ocurre en este día si:
            // - La fecha del día está entre startDate y endDate, O
            // - El evento comienza en este día, O
            // - El evento termina en este día
            return (dayStart >= eventStart && dayStart <= eventEnd) ||
                   (dayEnd >= eventStart && dayEnd <= eventEnd) ||
                   (eventStart <= dayEnd && eventEnd >= dayStart);
        });

        // ✅ AGREGAR EVENTOS AL DÍA DEL CALENDARIO
        if (dayEvents.length > 0) {
            dayDiv.classList.add('has-events');

            const eventsList = document.createElement('div');
            eventsList.className = 'day-events';

            dayEvents.forEach(event => {
                const eventDiv = document.createElement('div');
                eventDiv.className = 'calendar-event';
                eventDiv.textContent = event.title;
                eventDiv.title = `${event.title}\n${event.description || ''}\nInicio: ${event.startDate.toLocaleString()}\nFin: ${event.endDate.toLocaleString()}`;
                eventsList.appendChild(eventDiv);
            });

            dayDiv.appendChild(eventsList);
        }

        // Agregar número del día al final
        const dayNumberDiv = document.createElement('div');
        dayNumberDiv.className = 'day-number';
        dayNumberDiv.textContent = dayNumber;
        dayDiv.appendChild(dayNumberDiv);

        grid.appendChild(dayDiv);
    }
}

async function prevMonth() {
    currentDate.setMonth(currentDate.getMonth() - 1);
    clearCalendarEvents();
    // Recargar eventos para el nuevo mes
    if (userId) {
        await loadEvents();
    } else {
        renderCalendar();
    }
}

async function nextMonth() {
    currentDate.setMonth(currentDate.getMonth() + 1);
    clearCalendarEvents();
    // Recargar eventos para el nuevo mes
    if (userId) {
        await loadEvents();
    } else {
        renderCalendar();
    }
}

// ✅ LIMPIAR EVENTOS DEL CALENDARIO CUANDO SE CAMBIA DE MES
function clearCalendarEvents() {
    calendarEvents = [];
    console.log('🧹 Calendar events cleared');
}

// ✅ FUNCIÓN DE DEBUG MEJORADA
function debugState() {
    console.log('🐛 Current State:', {
        token: token ? `SET (${token.substring(0, 10)}...)` : 'MISSING',
        userId: userId || 'MISSING',
        localStorage: {
            token: localStorage.getItem('agenda_token') ? 'SET' : 'MISSING',
            userId: localStorage.getItem('agenda_userId') || 'MISSING'
        }
    });
}

// Initialize
window.onload = () => {
    console.log('🚀 [DEBUG] App initializing...');
    console.log('📱 [DEBUG] About to call loadSession');
    loadSession();
    console.log('📅 [DEBUG] About to call renderCalendar');
    renderCalendar();

    // Debug cada 10 segundos
    setInterval(debugState, 10000);

    console.log('✅ [DEBUG] App initialization completed');
};

// Delete event function
async function deleteEvent(eventId) {
    if (!confirm('¿Estás seguro de que quieres eliminar este evento?')) {
        return;
    }

    try {
        console.log('🗑️ Deleting event:', eventId);

        // ✅ CONSTRUIR URL MANUALMENTE CON user_id PARA DELETE
        const deleteUrl = `/events/${eventId}?user_id=${encodeURIComponent(userId)}`;
        const result = await apiRequest(deleteUrl, 'DELETE');
        showNotification('Evento eliminado exitosamente!', 'success');

        // Recargar eventos (renderCalendar ya se llama dentro de loadEvents)
        await loadEvents();
    } catch (error) {
        console.error('❌ Failed to delete event:', error);
        showNotification('Error al eliminar evento: ' + error.message, 'error');
    }
}

// Delete account functions
function showDeleteAccountModal() {
    showModal('delete-account-modal');
}

async function deleteAccount() {
    if (!confirm('¿Estás completamente seguro? Esta acción no se puede deshacer.')) {
        return;
    }

    try {
        console.log('🗑️ Deleting user account');

        // ✅ CONSTRUIR URL MANUALMENTE CON user_id PARA DELETE
        const deleteUrl = `/auth/account?user_id=${encodeURIComponent(userId)}`;
        const result = await apiRequest(deleteUrl, 'DELETE');
        showNotification('Cuenta eliminada exitosamente. Redirigiendo...', 'success');

        // Clear session and redirect to login
        clearSession();
        setTimeout(() => {
            location.reload();
        }, 2000);
    } catch (error) {
        console.error('❌ Failed to delete account:', error);
        showNotification('Error al eliminar cuenta: ' + error.message, 'error');
    }
}

// Close modals when clicking outside
window.onclick = (event) => {
    if (event.target.classList.contains('modal')) {
        event.target.style.display = 'none';
    }
};