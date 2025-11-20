let token = null;
let userId = null;
let currentDate = new Date();

// API request helper
async function apiRequest(endpoint, method = 'GET', body = null) {
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const response = await fetch(`/api${endpoint}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : null
    });

    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `HTTP error! status: ${response.status}`);
    }

    return response.json();
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
        const result = await apiRequest('/auth/register', 'POST', { username, email, password });
        showNotification('Usuario registrado exitosamente!', 'success');
        showTab('login');
        document.getElementById('reg-username').value = '';
        document.getElementById('reg-email').value = '';
        document.getElementById('reg-password').value = '';
    } catch (error) {
        showNotification('Error en el registro: ' + error.message, 'error');
    }
}

async function login(event) {
    event.preventDefault();

    const email = document.getElementById('login-email').value;
    const password = document.getElementById('login-password').value;

    try {
        const result = await apiRequest('/auth/login', 'POST', { email, password });
        token = result.token;
        userId = result.user_id;

        // Update UI
        document.getElementById('auth-section').style.display = 'none';
        document.getElementById('dashboard').style.display = 'block';
        document.getElementById('user-info').style.display = 'flex';
        document.getElementById('user-email').textContent = email;

        showNotification('Sesión iniciada exitosamente!', 'success');

        // Load data
        loadEvents();
        loadGroups();
        renderCalendar();

        // Clear form
        document.getElementById('login-email').value = '';
        document.getElementById('login-password').value = '';

    } catch (error) {
        showNotification('Error al iniciar sesión: ' + error.message, 'error');
    }
}

function logout() {
    token = null;
    userId = null;

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
async function createEvent(event) {
    event.preventDefault();

    const title = document.getElementById('event-title').value;
    const description = document.getElementById('event-description').value;
    const startTime = document.getElementById('event-start').value;
    const endTime = document.getElementById('event-end').value;
    const groupId = document.getElementById('event-group').value;

    try {
        const result = await apiRequest('/events', 'POST', {
            title,
            description,
            start_time: new Date(startTime).toISOString(),
            end_time: new Date(endTime).toISOString(),
            user_id: userId,
            group_id: groupId || undefined
        });

        showNotification('Evento creado exitosamente!', 'success');
        closeModal('event-modal');

        // Clear form
        document.getElementById('event-title').value = '';
        document.getElementById('event-description').value = '';
        document.getElementById('event-start').value = '';
        document.getElementById('event-end').value = '';

        loadEvents();
        renderCalendar();

    } catch (error) {
        showNotification('Error al crear evento: ' + error.message, 'error');
    }
}

async function loadEvents() {
    try {
        const result = await apiRequest('/events');
        const container = document.getElementById('events-list');
        container.innerHTML = '';

        if (result.events && result.events.length > 0) {
            result.events.forEach(event => {
                const eventCard = document.createElement('div');
                eventCard.className = 'item-card';
                eventCard.innerHTML = `
                    <h4>${event.title || 'Sin título'}</h4>
                    <p>${event.description || 'Sin descripción'}</p>
                    <div class="date">Inicio: ${new Date(event.start_time * 1000).toLocaleString()}</div>
                    <div class="date">Fin: ${new Date(event.end_time * 1000).toLocaleString()}</div>
                `;
                container.appendChild(eventCard);
            });
        } else {
            container.innerHTML = '<p>No hay eventos para mostrar</p>';
        }
    } catch (error) {
        console.error('Failed to load events:', error);
        showNotification('Error al cargar eventos', 'error');
    }
}

// Group functions
async function createGroup(event) {
    event.preventDefault();

    const name = document.getElementById('group-name').value;
    const description = document.getElementById('group-description').value;
    const isHierarchical = document.getElementById('group-hierarchical').checked;

    try {
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
        showNotification('Error al crear grupo: ' + error.message, 'error');
    }
}

async function loadGroups() {
    try {
        const result = await apiRequest('/groups');
        const container = document.getElementById('groups-list');
        const groupSelect = document.getElementById('event-group');

        container.innerHTML = '';
        groupSelect.innerHTML = '<option value="">Sin grupo</option>';

        if (result.groups && result.groups.length > 0) {
            result.groups.forEach(group => {
                // Add to list
                const groupCard = document.createElement('div');
                groupCard.className = 'item-card';
                groupCard.innerHTML = `
                    <h4>${group.name || 'Sin nombre'}</h4>
                    <p>${group.description || 'Sin descripción'}</p>
                    <p>Tipo: ${group.is_hierarchical ? 'Jerárquico' : 'No jerárquico'}</p>
                `;
                container.appendChild(groupCard);

                // Add to select
                const option = document.createElement('option');
                option.value = group.id;
                option.textContent = group.name;
                groupSelect.appendChild(option);
            });
        } else {
            container.innerHTML = '<p>No hay grupos para mostrar</p>';
        }
    } catch (error) {
        console.error('Failed to load groups:', error);
        showNotification('Error al cargar grupos', 'error');
    }
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

        dayDiv.textContent = dayNumber;

        // Add event indicator (placeholder - would need actual events data)
        // if (hasEventOnDay(dayDate)) {
        //     const indicator = document.createElement('div');
        //     indicator.className = 'event-indicator';
        //     dayDiv.appendChild(indicator);
        // }

        grid.appendChild(dayDiv);
    }
}

function prevMonth() {
    currentDate.setMonth(currentDate.getMonth() - 1);
    renderCalendar();
}

function nextMonth() {
    currentDate.setMonth(currentDate.getMonth() + 1);
    renderCalendar();
}

// Initialize
window.onload = () => {
    renderCalendar();
};

// Close modals when clicking outside
window.onclick = (event) => {
    if (event.target.classList.contains('modal')) {
        event.target.style.display = 'none';
    }
};