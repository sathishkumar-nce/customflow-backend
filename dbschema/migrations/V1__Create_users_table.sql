-- =================================================================
-- V1__Create_users_table.sql
-- Migration: Create users table with authentication and role management
-- =================================================================

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'editor',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_users_role CHECK (role IN ('editor', 'viewer', 'admin')),
    CONSTRAINT chk_users_email_format CHECK (email ~ '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- Create indexes for performance
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Create trigger function to automatically update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default admin user (password: admin123)
INSERT INTO users (username, email, password, role) VALUES 
('admin', 'admin@customflow.com', '$2a$10$N9qo8uLOickgx2ZMRZoMye1P8xhHFgZrb0EbvFqKz4h5zYPbqZwne', 'admin');


-- =================================================================
-- V2__Create_orders_table.sql
-- Migration: Create orders table for custom table cover orders
-- =================================================================

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(100) NOT NULL UNIQUE,
    customer_name VARCHAR(255),
    source VARCHAR(20) NOT NULL DEFAULT 'amazon',
    phone_number VARCHAR(20),
    length DECIMAL(10,2) NOT NULL,
    width DECIMAL(10,2) NOT NULL,
    thickness VARCHAR(10) NOT NULL DEFAULT '3mm',
    corner_style VARCHAR(20) NOT NULL DEFAULT 'sharp',
    notes TEXT,
    special_notes TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'new',
    created_by INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_orders_source CHECK (source IN ('amazon', 'whatsapp', 'sms', 'call')),
    CONSTRAINT chk_orders_corner_style CHECK (corner_style IN ('sharp', 'rounded', 'custom')),
    CONSTRAINT chk_orders_status CHECK (status IN ('new', 'in-progress', 'done')),
    CONSTRAINT chk_orders_length CHECK (length > 0),
    CONSTRAINT chk_orders_width CHECK (width > 0),
    CONSTRAINT chk_orders_thickness CHECK (thickness IN ('2mm', '3mm', '5mm', '8mm')),
    CONSTRAINT fk_orders_created_by FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Create indexes for performance
CREATE INDEX idx_orders_order_id ON orders(order_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_source ON orders(source);
CREATE INDEX idx_orders_created_by ON orders(created_by);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX idx_orders_customer_name ON orders(customer_name);
CREATE INDEX idx_orders_status_created_at ON orders(status, created_at DESC);

-- Create trigger for updated_at
CREATE TRIGGER update_orders_updated_at 
    BEFORE UPDATE ON orders 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();



    -- =================================================================
-- V3__Create_order_images_table.sql
-- Migration: Create order images table for file attachments
-- =================================================================

CREATE TABLE order_images (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL,
    filename VARCHAR(255) NOT NULL,
    path VARCHAR(500) NOT NULL,
    size BIGINT,
    mime_type VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_order_images_order_id FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX idx_order_images_order_id ON order_images(order_id);
CREATE INDEX idx_order_images_filename ON order_images(filename);
CREATE INDEX idx_order_images_created_at ON order_images(created_at DESC);



-- =================================================================
-- V4__Create_ai_responses_table.sql
-- Migration: Create AI responses table for tracking AI interactions
-- =================================================================

CREATE TABLE ai_responses (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    input_message TEXT NOT NULL,
    response TEXT NOT NULL,
    tone VARCHAR(20) DEFAULT 'friendly',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_ai_responses_tone CHECK (tone IN ('friendly', 'formal', 'short')),
    CONSTRAINT fk_ai_responses_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX idx_ai_responses_user_id ON ai_responses(user_id);
CREATE INDEX idx_ai_responses_tone ON ai_responses(tone);
CREATE INDEX idx_ai_responses_created_at ON ai_responses(created_at DESC);