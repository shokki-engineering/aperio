-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "catalog";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "document";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "event";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "finance";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "guest";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "operations";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "pricing";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "sales";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "stay";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "venue";

-- CreateExtension
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- CreateEnum
CREATE TYPE "catalog"."course" AS ENUM ('WELCOME_DRINK', 'STARTER', 'SOUP', 'SALAD', 'MAIN', 'BREAD', 'RICE', 'SIDE', 'DESSERT', 'BEVERAGE', 'LIVE_COUNTER', 'OTHER');

-- CreateEnum
CREATE TYPE "document"."category" AS ENUM ('FLOOR_PLAN', 'PARKING', 'IT_NETWORK', 'SAFETY', 'LICENCE', 'INSURANCE', 'CONTRACT', 'IDENTITY', 'MENU', 'BROCHURE', 'MEDIA', 'OTHER');

-- CreateEnum
CREATE TYPE "document"."visibility" AS ENUM ('STAFF', 'GUEST', 'PUBLIC');

-- CreateEnum
CREATE TYPE "event"."event_state" AS ENUM ('TENTATIVE', 'CONFIRMED', 'COMPLETED', 'CANCELLED');

-- CreateEnum
CREATE TYPE "finance"."invoice_kind" AS ENUM ('TAX_INVOICE', 'PROFORMA', 'RECEIPT_VOUCHER', 'CREDIT_NOTE');

-- CreateEnum
CREATE TYPE "finance"."payment_kind" AS ENUM ('RECEIPT', 'REFUND');

-- CreateEnum
CREATE TYPE "finance"."payment_method" AS ENUM ('GATEWAY', 'UPI', 'CARD', 'CASH', 'BANK_TRANSFER', 'CHEQUE', 'CHANNEL', 'OTHER');

-- CreateEnum
CREATE TYPE "finance"."invoice_state" AS ENUM ('DRAFT', 'ISSUED', 'PAID', 'CANCELLED');

-- CreateEnum
CREATE TYPE "finance"."payment_state" AS ENUM ('PENDING', 'SUCCEEDED', 'FAILED', 'VOIDED');

-- CreateEnum
CREATE TYPE "guest"."contact_kind" AS ENUM ('PERSON', 'COMPANY');

-- CreateEnum
CREATE TYPE "operations"."department" AS ENUM ('PANTRY', 'KITCHEN', 'HOUSEKEEPING', 'MAINTENANCE', 'IT_AV', 'FRONT_DESK', 'BANQUET', 'SECURITY');

-- CreateEnum
CREATE TYPE "operations"."priority" AS ENUM ('LOW', 'NORMAL', 'URGENT');

-- CreateEnum
CREATE TYPE "operations"."operations_state" AS ENUM ('OPEN', 'ACKNOWLEDGED', 'IN_PROGRESS', 'DONE', 'CANCELLED');

-- CreateEnum
CREATE TYPE "pricing"."price_target" AS ENUM ('MENU', 'FIXED_PACKAGE', 'SPACE', 'DISH');

-- CreateEnum
CREATE TYPE "pricing"."price_rule_adjustment_case" AS ENUM ('FIXED_PRICE', 'PERCENT', 'AMOUNT');

-- CreateEnum
CREATE TYPE "sales"."lead_kind" AS ENUM ('STAY', 'EVENT', 'EVENT_WITH_STAY');

-- CreateEnum
CREATE TYPE "sales"."stage" AS ENUM ('NEW', 'CONTACTED', 'SITE_VISIT', 'PROPOSAL', 'NEGOTIATION', 'WON', 'LOST');

-- CreateEnum
CREATE TYPE "sales"."lost_reason" AS ENUM ('PRICE', 'DATES', 'COMPETITOR', 'CAPACITY', 'CANCELLED_PLANS', 'NO_RESPONSE', 'OTHER');

-- CreateEnum
CREATE TYPE "sales"."activity_kind" AS ENUM ('CALL', 'EMAIL', 'WHATSAPP', 'MEETING', 'SITE_VISIT', 'QUOTE_SENT', 'NOTE');

-- CreateEnum
CREATE TYPE "sales"."sales_state" AS ENUM ('DRAFT', 'SENT', 'ACCEPTED', 'DECLINED', 'EXPIRED', 'SUPERSEDED');

-- CreateEnum
CREATE TYPE "stay"."stay_state" AS ENUM ('PENDING', 'CONFIRMED', 'CHECKED_IN', 'CHECKED_OUT', 'CANCELLED', 'NO_SHOW');

-- CreateEnum
CREATE TYPE "event"."event_type" AS ENUM ('WEDDING', 'RECEPTION', 'ENGAGEMENT', 'PRE_WEDDING', 'BIRTHDAY', 'ANNIVERSARY', 'SOCIAL', 'RELIGIOUS', 'MEETING', 'CONFERENCE', 'OFFSITE', 'LAUNCH', 'OTHER');

-- CreateEnum
CREATE TYPE "finance"."channel" AS ENUM ('WEBSITE', 'PHONE', 'WALK_IN', 'EMAIL', 'WHATSAPP', 'INSTAGRAM', 'FACEBOOK', 'REFERRAL', 'BOOKING_COM', 'AIRBNB', 'MAKEMYTRIP', 'GOIBIBO', 'AGODA', 'EXPEDIA', 'CHANNEL_MANAGER', 'RFP', 'OTHER');

-- CreateEnum
CREATE TYPE "event"."layout" AS ENUM ('THEATRE', 'BANQUET', 'CLASSROOM', 'U_SHAPE', 'BOARDROOM', 'COCKTAIL', 'CABARET', 'FLOOR');

-- CreateEnum
CREATE TYPE "venue"."venue_kind" AS ENUM ('STAYS', 'EVENTS', 'STAYS_AND_EVENTS', 'RESTAURANT');

-- CreateEnum
CREATE TYPE "venue"."space_kind" AS ENUM ('ROOM_TYPE', 'UNIT', 'HALL', 'OUTDOOR', 'MEETING_ROOM', 'DINING', 'TABLE');

-- CreateTable
CREATE TABLE "catalog"."dishes" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "course" "catalog"."course" NOT NULL DEFAULT 'WELCOME_DRINK',
    "cuisine" TEXT,
    "diets" TEXT,
    "allergens" TEXT,
    "price" JSONB,
    "tax" JSONB,
    "image_uri" TEXT,
    "available" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "dishes_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "catalog"."menus" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "diets" TEXT,
    "courses" JSONB NOT NULL,
    "add_ons" JSONB,
    "plate_price" JSONB NOT NULL,
    "min_plates" INTEGER,
    "tax" JSONB NOT NULL,
    "active" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "menus_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "catalog"."fixed_packages" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "duration" TEXT,
    "menu" TEXT,
    "inclusions" JSONB,
    "included_guests" INTEGER NOT NULL,
    "price" JSONB NOT NULL,
    "extra_guest_price" JSONB,
    "tax" JSONB NOT NULL,
    "active" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "fixed_packages_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "catalog"."fixed_package_spaces" (
    "id" TEXT NOT NULL,
    "fixed_package_id" TEXT NOT NULL,
    "space_id" TEXT NOT NULL,
    "space_name" TEXT NOT NULL,

    CONSTRAINT "fixed_package_spaces_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "document"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "category" "document"."category" NOT NULL DEFAULT 'FLOOR_PLAN',
    "subject" TEXT,
    "uri" TEXT NOT NULL,
    "mime_type" TEXT,
    "size_bytes" BIGINT,
    "sha256" TEXT,
    "visibility" "document"."visibility",
    "expiry_date" DATE,
    "tags" TEXT[],
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "document_resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "event"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "lead" TEXT,
    "event_type" "event"."event_type" NOT NULL DEFAULT 'WEDDING',
    "state" "event"."event_state" DEFAULT 'TENTATIVE',
    "dates" JSONB NOT NULL,
    "guests" INTEGER,
    "quote" TEXT,
    "special_requests" JSONB,
    "coordinator" TEXT,
    "notes" TEXT,
    "cancel_reason" TEXT,
    "cancel_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "event_resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "event"."sessions" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "event_type" "event"."event_type",
    "space" TEXT NOT NULL,
    "start_time" TIMESTAMPTZ(6) NOT NULL,
    "end_time" TIMESTAMPTZ(6) NOT NULL,
    "setup_time" TIMESTAMPTZ(6),
    "teardown_time" TIMESTAMPTZ(6),
    "layout" "event"."layout",
    "guests" INTEGER NOT NULL,
    "guaranteed_guests" INTEGER,
    "menu" TEXT,
    "dietary_requirements" JSONB,
    "special_requests" JSONB,
    "freebusy_reservation" TEXT,
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,
    "event_id" TEXT NOT NULL,

    CONSTRAINT "sessions_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "finance"."invoices" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "number" TEXT,
    "kind" "finance"."invoice_kind" NOT NULL DEFAULT 'TAX_INVOICE',
    "state" "finance"."invoice_state" DEFAULT 'DRAFT',
    "contact" TEXT NOT NULL,
    "buyer_gst" JSONB,
    "billing_address" JSONB,
    "booking" TEXT,
    "quote" TEXT,
    "corrects" TEXT,
    "supply_state_code" TEXT NOT NULL,
    "lines" JSONB NOT NULL,
    "taxable_total" JSONB,
    "cgst" JSONB,
    "sgst" JSONB,
    "igst" JSONB,
    "total" JSONB,
    "paid" JSONB,
    "due" JSONB,
    "issue_date" DATE,
    "due_date" DATE,
    "irn" TEXT,
    "notes" TEXT,
    "cancel_reason" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "invoices_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "finance"."payments" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "invoice" TEXT,
    "booking" TEXT,
    "kind" "finance"."payment_kind" NOT NULL DEFAULT 'RECEIPT',
    "method" "finance"."payment_method" NOT NULL DEFAULT 'GATEWAY',
    "channel" "finance"."channel",
    "amount" JSONB NOT NULL,
    "exchange_rate" DECIMAL,
    "base_amount" JSONB,
    "state" "finance"."payment_state" DEFAULT 'PENDING',
    "gateway" TEXT,
    "gateway_reference" TEXT,
    "reference" TEXT,
    "receive_time" TIMESTAMPTZ(6),
    "recorder" TEXT,
    "refunds" TEXT,
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "payments_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "guest"."contacts" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "kind" "guest"."contact_kind" NOT NULL DEFAULT 'PERSON',
    "display_name" TEXT NOT NULL,
    "name_parts" JSONB,
    "company_display_name" TEXT,
    "phones" JSONB,
    "emails" TEXT[],
    "addresses" JSONB,
    "gst" JSONB,
    "dietary_requirements" JSONB,
    "marketing_consent" BOOLEAN,
    "tags" TEXT[],
    "notes" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "phone_numbers" TEXT[],

    CONSTRAINT "contacts_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "operations"."service_requests" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "summary" TEXT NOT NULL,
    "department" "operations"."department" NOT NULL DEFAULT 'PANTRY',
    "space" TEXT,
    "location" TEXT,
    "items" JSONB,
    "priority" "operations"."priority",
    "due_time" TIMESTAMPTZ(6),
    "requester" TEXT,
    "assignee" TEXT,
    "booking" TEXT,
    "charge" JSONB,
    "state" "operations"."operations_state" DEFAULT 'OPEN',
    "notes" TEXT,
    "acknowledge_time" TIMESTAMPTZ(6),
    "complete_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "service_requests_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "pricing"."seasons" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "dates" JSONB,
    "recurs_yearly" BOOLEAN,
    "weekdays" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "seasons_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "pricing"."price_rules" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "season" TEXT,
    "target" "pricing"."price_target" NOT NULL DEFAULT 'MENU',
    "min_guests" INTEGER,
    "fixed_price" JSONB,
    "percent" DECIMAL,
    "amount" JSONB,
    "priority" INTEGER,
    "active" BOOLEAN,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "adjustment_case" "pricing"."price_rule_adjustment_case",
    "venue_id" TEXT NOT NULL,
    "items" TEXT[],

    CONSTRAINT "price_rules_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."leads" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "venue" TEXT,
    "kind" "sales"."lead_kind" NOT NULL DEFAULT 'STAY',
    "event_type" "event"."event_type",
    "source" "finance"."channel",
    "source_detail" TEXT,
    "stage" "sales"."stage" DEFAULT 'NEW',
    "dates" JSONB,
    "dates_flexible" BOOLEAN,
    "guests" INTEGER,
    "rooms" INTEGER,
    "budget" JSONB,
    "requirements" TEXT,
    "owner" TEXT,
    "next_follow_up_time" TIMESTAMPTZ(6),
    "lost_reason" "sales"."lost_reason",
    "lost_detail" TEXT,
    "hold" TEXT,
    "bookings" TEXT[],
    "close_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,

    CONSTRAINT "leads_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."activities" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "kind" "sales"."activity_kind" NOT NULL DEFAULT 'CALL',
    "occur_time" TIMESTAMPTZ(6) NOT NULL,
    "summary" TEXT NOT NULL,
    "details" TEXT,
    "author" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "lead_id" TEXT NOT NULL,

    CONSTRAINT "activities_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "sales"."quotes" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "lead" TEXT,
    "supersedes" TEXT,
    "state" "sales"."sales_state" DEFAULT 'DRAFT',
    "event_type" "event"."event_type",
    "dates" JSONB NOT NULL,
    "guests" INTEGER,
    "lines" JSONB NOT NULL,
    "supply_state_code" TEXT,
    "subtotal" JSONB,
    "tax_total" JSONB,
    "total" JSONB,
    "advance" JSONB,
    "terms" TEXT,
    "expire_time" TIMESTAMPTZ(6),
    "send_time" TIMESTAMPTZ(6),
    "accept_time" TIMESTAMPTZ(6),
    "booking" TEXT,
    "decline_reason" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "ttl" TEXT,
    "venue_id" TEXT NOT NULL,

    CONSTRAINT "quotes_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "stay"."reservations" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "contact" TEXT NOT NULL,
    "lead" TEXT,
    "event" TEXT,
    "channel" "finance"."channel" NOT NULL DEFAULT 'WEBSITE',
    "channel_reference" TEXT,
    "state" "stay"."stay_state" DEFAULT 'PENDING',
    "arrival_date" DATE NOT NULL,
    "departure_date" DATE NOT NULL,
    "adults" INTEGER NOT NULL,
    "children" INTEGER,
    "rooms" JSONB NOT NULL,
    "room_stays" JSONB,
    "res_guests" JSONB,
    "dietary_requirements" JSONB,
    "special_requests" JSONB,
    "total" JSONB,
    "quote" TEXT,
    "notes" TEXT,
    "cancel_reason" TEXT,
    "arrive_time" TIMESTAMPTZ(6),
    "depart_time" TIMESTAMPTZ(6),
    "cancel_time" TIMESTAMPTZ(6),
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,
    "freebusy_reservations" TEXT[],

    CONSTRAINT "reservations_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "venue"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "kind" "venue"."venue_kind" NOT NULL DEFAULT 'STAYS',
    "address" JSONB NOT NULL,
    "time_zone" TEXT NOT NULL,
    "currency_code" TEXT NOT NULL,
    "gst" JSONB NOT NULL,
    "hotel_info" JSONB,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "invoice_prefix" TEXT,

    CONSTRAINT "venue_resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "venue"."spaces" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "kind" "venue"."space_kind" NOT NULL DEFAULT 'ROOM_TYPE',
    "capacity" JSONB,
    "inventory_code" TEXT,
    "create_time" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMPTZ(6) NOT NULL,
    "etag" TEXT,
    "venue_id" TEXT NOT NULL,
    "resources" TEXT[],

    CONSTRAINT "spaces_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "dishes_name_key" ON "catalog"."dishes"("name");

-- CreateIndex
CREATE INDEX "dishes_update_time_idx" ON "catalog"."dishes"("update_time");

-- CreateIndex
CREATE INDEX "dishes_venue_id_id_idx" ON "catalog"."dishes"("venue_id", "id");

-- CreateIndex
CREATE INDEX "dishes_display_name_idx" ON "catalog"."dishes" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "dishes_cuisine_idx" ON "catalog"."dishes" USING GIN ("cuisine" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "menus_name_key" ON "catalog"."menus"("name");

-- CreateIndex
CREATE INDEX "menus_update_time_idx" ON "catalog"."menus"("update_time");

-- CreateIndex
CREATE INDEX "menus_venue_id_id_idx" ON "catalog"."menus"("venue_id", "id");

-- CreateIndex
CREATE INDEX "menus_display_name_idx" ON "catalog"."menus" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "fixed_packages_name_key" ON "catalog"."fixed_packages"("name");

-- CreateIndex
CREATE INDEX "fixed_packages_update_time_idx" ON "catalog"."fixed_packages"("update_time");

-- CreateIndex
CREATE INDEX "fixed_packages_venue_id_id_idx" ON "catalog"."fixed_packages"("venue_id", "id");

-- CreateIndex
CREATE INDEX "fixed_packages_menu_idx" ON "catalog"."fixed_packages"("menu");

-- CreateIndex
CREATE INDEX "fixed_packages_display_name_idx" ON "catalog"."fixed_packages" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "fixed_package_spaces_space_id_idx" ON "catalog"."fixed_package_spaces"("space_id");

-- CreateIndex
CREATE UNIQUE INDEX "fixed_package_spaces_fixed_package_id_space_id_key" ON "catalog"."fixed_package_spaces"("fixed_package_id", "space_id");

-- CreateIndex
CREATE UNIQUE INDEX "document_resource_name_key" ON "document"."resource"("name");

-- CreateIndex
CREATE INDEX "document_resource_update_time_idx" ON "document"."resource"("update_time");

-- CreateIndex
CREATE INDEX "document_resource_category_update_time_idx" ON "document"."resource"("category", "update_time");

-- CreateIndex
CREATE INDEX "document_resource_subject_idx" ON "document"."resource"("subject");

-- CreateIndex
CREATE INDEX "document_resource_venue_id_id_idx" ON "document"."resource"("venue_id", "id");

-- CreateIndex
CREATE INDEX "resource_display_name_idx" ON "document"."resource" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "resource_tags_idx" ON "document"."resource" USING GIN ("tags");

-- CreateIndex
CREATE UNIQUE INDEX "event_resource_name_key" ON "event"."resource"("name");

-- CreateIndex
CREATE INDEX "event_resource_update_time_idx" ON "event"."resource"("update_time");

-- CreateIndex
CREATE INDEX "event_resource_state_update_time_idx" ON "event"."resource"("state", "update_time");

-- CreateIndex
CREATE INDEX "event_resource_coordinator_idx" ON "event"."resource"("coordinator");

-- CreateIndex
CREATE INDEX "event_resource_venue_id_id_idx" ON "event"."resource"("venue_id", "id");

-- CreateIndex
CREATE INDEX "event_resource_contact_idx" ON "event"."resource"("contact");

-- CreateIndex
CREATE INDEX "event_resource_lead_idx" ON "event"."resource"("lead");

-- CreateIndex
CREATE INDEX "event_resource_quote_idx" ON "event"."resource"("quote");

-- CreateIndex
CREATE INDEX "resource_display_name_idx" ON "event"."resource" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "sessions_name_key" ON "event"."sessions"("name");

-- CreateIndex
CREATE INDEX "sessions_update_time_idx" ON "event"."sessions"("update_time");

-- CreateIndex
CREATE INDEX "sessions_start_time_end_time_idx" ON "event"."sessions"("start_time", "end_time");

-- CreateIndex
CREATE INDEX "sessions_freebusy_reservation_idx" ON "event"."sessions"("freebusy_reservation");

-- CreateIndex
CREATE INDEX "sessions_event_id_id_idx" ON "event"."sessions"("event_id", "id");

-- CreateIndex
CREATE INDEX "sessions_space_idx" ON "event"."sessions"("space");

-- CreateIndex
CREATE INDEX "sessions_menu_idx" ON "event"."sessions"("menu");

-- CreateIndex
CREATE INDEX "sessions_venue_id_idx" ON "event"."sessions"("venue_id");

-- CreateIndex
CREATE UNIQUE INDEX "invoices_name_key" ON "finance"."invoices"("name");

-- CreateIndex
CREATE INDEX "invoices_update_time_idx" ON "finance"."invoices"("update_time");

-- CreateIndex
CREATE INDEX "invoices_state_issue_date_idx" ON "finance"."invoices"("state", "issue_date");

-- CreateIndex
CREATE INDEX "invoices_booking_idx" ON "finance"."invoices"("booking");

-- CreateIndex
CREATE INDEX "invoices_venue_id_id_idx" ON "finance"."invoices"("venue_id", "id");

-- CreateIndex
CREATE INDEX "invoices_contact_idx" ON "finance"."invoices"("contact");

-- CreateIndex
CREATE INDEX "invoices_quote_idx" ON "finance"."invoices"("quote");

-- CreateIndex
CREATE INDEX "invoices_number_idx" ON "finance"."invoices" USING GIN ("number" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "invoices_irn_idx" ON "finance"."invoices" USING GIN ("irn" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "invoices_number_key" ON "finance"."invoices"("number");

-- CreateIndex
CREATE UNIQUE INDEX "payments_name_key" ON "finance"."payments"("name");

-- CreateIndex
CREATE INDEX "payments_update_time_idx" ON "finance"."payments"("update_time");

-- CreateIndex
CREATE INDEX "payments_state_receive_time_idx" ON "finance"."payments"("state", "receive_time");

-- CreateIndex
CREATE INDEX "payments_booking_idx" ON "finance"."payments"("booking");

-- CreateIndex
CREATE INDEX "payments_recorder_idx" ON "finance"."payments"("recorder");

-- CreateIndex
CREATE INDEX "payments_venue_id_id_idx" ON "finance"."payments"("venue_id", "id");

-- CreateIndex
CREATE INDEX "payments_contact_idx" ON "finance"."payments"("contact");

-- CreateIndex
CREATE INDEX "payments_invoice_idx" ON "finance"."payments"("invoice");

-- CreateIndex
CREATE INDEX "payments_gateway_reference_idx" ON "finance"."payments" USING GIN ("gateway_reference" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "payments_reference_idx" ON "finance"."payments" USING GIN ("reference" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "payments_gateway_gateway_reference_key" ON "finance"."payments"("gateway", "gateway_reference");

-- CreateIndex
CREATE UNIQUE INDEX "contacts_name_key" ON "guest"."contacts"("name");

-- CreateIndex
CREATE INDEX "contacts_update_time_idx" ON "guest"."contacts"("update_time");

-- CreateIndex
CREATE INDEX "contacts_display_name_idx" ON "guest"."contacts" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "contacts_company_display_name_idx" ON "guest"."contacts" USING GIN ("company_display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "contacts_emails_idx" ON "guest"."contacts" USING GIN ("emails");

-- CreateIndex
CREATE INDEX "contacts_tags_idx" ON "guest"."contacts" USING GIN ("tags");

-- CreateIndex
CREATE INDEX "contacts_phone_numbers_idx" ON "guest"."contacts" USING GIN ("phone_numbers");

-- CreateIndex
CREATE UNIQUE INDEX "service_requests_name_key" ON "operations"."service_requests"("name");

-- CreateIndex
CREATE INDEX "service_requests_update_time_idx" ON "operations"."service_requests"("update_time");

-- CreateIndex
CREATE INDEX "service_requests_department_state_due_time_idx" ON "operations"."service_requests"("department", "state", "due_time");

-- CreateIndex
CREATE INDEX "service_requests_requester_idx" ON "operations"."service_requests"("requester");

-- CreateIndex
CREATE INDEX "service_requests_assignee_idx" ON "operations"."service_requests"("assignee");

-- CreateIndex
CREATE INDEX "service_requests_booking_idx" ON "operations"."service_requests"("booking");

-- CreateIndex
CREATE INDEX "service_requests_venue_id_id_idx" ON "operations"."service_requests"("venue_id", "id");

-- CreateIndex
CREATE INDEX "service_requests_space_idx" ON "operations"."service_requests"("space");

-- CreateIndex
CREATE INDEX "service_requests_summary_idx" ON "operations"."service_requests" USING GIN ("summary" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "seasons_name_key" ON "pricing"."seasons"("name");

-- CreateIndex
CREATE INDEX "seasons_update_time_idx" ON "pricing"."seasons"("update_time");

-- CreateIndex
CREATE INDEX "seasons_venue_id_id_idx" ON "pricing"."seasons"("venue_id", "id");

-- CreateIndex
CREATE UNIQUE INDEX "price_rules_name_key" ON "pricing"."price_rules"("name");

-- CreateIndex
CREATE INDEX "price_rules_update_time_idx" ON "pricing"."price_rules"("update_time");

-- CreateIndex
CREATE INDEX "price_rules_venue_id_id_idx" ON "pricing"."price_rules"("venue_id", "id");

-- CreateIndex
CREATE INDEX "price_rules_season_idx" ON "pricing"."price_rules"("season");

-- CreateIndex
CREATE INDEX "price_rules_items_idx" ON "pricing"."price_rules" USING GIN ("items");

-- CreateIndex
CREATE UNIQUE INDEX "leads_name_key" ON "sales"."leads"("name");

-- CreateIndex
CREATE INDEX "leads_update_time_idx" ON "sales"."leads"("update_time");

-- CreateIndex
CREATE INDEX "leads_stage_next_follow_up_time_idx" ON "sales"."leads"("stage", "next_follow_up_time");

-- CreateIndex
CREATE INDEX "leads_owner_stage_idx" ON "sales"."leads"("owner", "stage");

-- CreateIndex
CREATE INDEX "leads_owner_idx" ON "sales"."leads"("owner");

-- CreateIndex
CREATE INDEX "leads_hold_idx" ON "sales"."leads"("hold");

-- CreateIndex
CREATE INDEX "leads_contact_idx" ON "sales"."leads"("contact");

-- CreateIndex
CREATE INDEX "leads_venue_idx" ON "sales"."leads"("venue");

-- CreateIndex
CREATE INDEX "leads_display_name_idx" ON "sales"."leads" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "leads_bookings_idx" ON "sales"."leads" USING GIN ("bookings");

-- CreateIndex
CREATE UNIQUE INDEX "activities_name_key" ON "sales"."activities"("name");

-- CreateIndex
CREATE INDEX "activities_update_time_idx" ON "sales"."activities"("update_time");

-- CreateIndex
CREATE INDEX "activities_author_idx" ON "sales"."activities"("author");

-- CreateIndex
CREATE INDEX "activities_lead_id_id_idx" ON "sales"."activities"("lead_id", "id");

-- CreateIndex
CREATE UNIQUE INDEX "quotes_name_key" ON "sales"."quotes"("name");

-- CreateIndex
CREATE INDEX "quotes_update_time_idx" ON "sales"."quotes"("update_time");

-- CreateIndex
CREATE INDEX "quotes_lead_create_time_idx" ON "sales"."quotes"("lead", "create_time");

-- CreateIndex
CREATE INDEX "quotes_state_expire_time_idx" ON "sales"."quotes"("state", "expire_time");

-- CreateIndex
CREATE INDEX "quotes_booking_idx" ON "sales"."quotes"("booking");

-- CreateIndex
CREATE INDEX "quotes_venue_id_id_idx" ON "sales"."quotes"("venue_id", "id");

-- CreateIndex
CREATE INDEX "quotes_contact_idx" ON "sales"."quotes"("contact");

-- CreateIndex
CREATE UNIQUE INDEX "reservations_name_key" ON "stay"."reservations"("name");

-- CreateIndex
CREATE INDEX "reservations_update_time_idx" ON "stay"."reservations"("update_time");

-- CreateIndex
CREATE INDEX "reservations_arrival_date_departure_date_idx" ON "stay"."reservations"("arrival_date", "departure_date");

-- CreateIndex
CREATE INDEX "reservations_channel_channel_reference_idx" ON "stay"."reservations"("channel", "channel_reference");

-- CreateIndex
CREATE INDEX "reservations_state_update_time_idx" ON "stay"."reservations"("state", "update_time");

-- CreateIndex
CREATE INDEX "reservations_venue_id_id_idx" ON "stay"."reservations"("venue_id", "id");

-- CreateIndex
CREATE INDEX "reservations_contact_idx" ON "stay"."reservations"("contact");

-- CreateIndex
CREATE INDEX "reservations_lead_idx" ON "stay"."reservations"("lead");

-- CreateIndex
CREATE INDEX "reservations_event_idx" ON "stay"."reservations"("event");

-- CreateIndex
CREATE INDEX "reservations_quote_idx" ON "stay"."reservations"("quote");

-- CreateIndex
CREATE INDEX "reservations_channel_reference_idx" ON "stay"."reservations" USING GIN ("channel_reference" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "reservations_freebusy_reservations_idx" ON "stay"."reservations" USING GIN ("freebusy_reservations");

-- CreateIndex
CREATE UNIQUE INDEX "venue_resource_name_key" ON "venue"."resource"("name");

-- CreateIndex
CREATE UNIQUE INDEX "venue_resource_invoice_prefix_key" ON "venue"."resource"("invoice_prefix");

-- CreateIndex
CREATE INDEX "venue_resource_update_time_idx" ON "venue"."resource"("update_time");

-- CreateIndex
CREATE INDEX "resource_display_name_idx" ON "venue"."resource" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE UNIQUE INDEX "spaces_name_key" ON "venue"."spaces"("name");

-- CreateIndex
CREATE INDEX "spaces_update_time_idx" ON "venue"."spaces"("update_time");

-- CreateIndex
CREATE INDEX "spaces_venue_id_id_idx" ON "venue"."spaces"("venue_id", "id");

-- CreateIndex
CREATE INDEX "spaces_display_name_idx" ON "venue"."spaces" USING GIN ("display_name" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "spaces_inventory_code_idx" ON "venue"."spaces" USING GIN ("inventory_code" gin_trgm_ops);

-- CreateIndex
CREATE INDEX "spaces_resources_idx" ON "venue"."spaces" USING GIN ("resources");

-- AddForeignKey
ALTER TABLE "catalog"."dishes" ADD CONSTRAINT "dishes_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "catalog"."menus" ADD CONSTRAINT "menus_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "catalog"."fixed_packages" ADD CONSTRAINT "fixed_packages_menu_fkey" FOREIGN KEY ("menu") REFERENCES "catalog"."menus"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "catalog"."fixed_packages" ADD CONSTRAINT "fixed_packages_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "catalog"."fixed_package_spaces" ADD CONSTRAINT "fixed_package_spaces_fixed_package_id_fkey" FOREIGN KEY ("fixed_package_id") REFERENCES "catalog"."fixed_packages"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "catalog"."fixed_package_spaces" ADD CONSTRAINT "fixed_package_spaces_space_id_fkey" FOREIGN KEY ("space_id") REFERENCES "venue"."spaces"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "document"."resource" ADD CONSTRAINT "document_resource_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."resource" ADD CONSTRAINT "event_resource_contact_fkey" FOREIGN KEY ("contact") REFERENCES "guest"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."resource" ADD CONSTRAINT "event_resource_lead_fkey" FOREIGN KEY ("lead") REFERENCES "sales"."leads"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."resource" ADD CONSTRAINT "event_resource_quote_fkey" FOREIGN KEY ("quote") REFERENCES "sales"."quotes"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."resource" ADD CONSTRAINT "event_resource_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."sessions" ADD CONSTRAINT "sessions_space_fkey" FOREIGN KEY ("space") REFERENCES "venue"."spaces"("name") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."sessions" ADD CONSTRAINT "sessions_menu_fkey" FOREIGN KEY ("menu") REFERENCES "catalog"."menus"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."sessions" ADD CONSTRAINT "sessions_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "event"."sessions" ADD CONSTRAINT "sessions_event_id_fkey" FOREIGN KEY ("event_id") REFERENCES "event"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."invoices" ADD CONSTRAINT "invoices_contact_fkey" FOREIGN KEY ("contact") REFERENCES "guest"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."invoices" ADD CONSTRAINT "invoices_quote_fkey" FOREIGN KEY ("quote") REFERENCES "sales"."quotes"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."invoices" ADD CONSTRAINT "invoices_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."payments" ADD CONSTRAINT "payments_contact_fkey" FOREIGN KEY ("contact") REFERENCES "guest"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."payments" ADD CONSTRAINT "payments_invoice_fkey" FOREIGN KEY ("invoice") REFERENCES "finance"."invoices"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "finance"."payments" ADD CONSTRAINT "payments_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "operations"."service_requests" ADD CONSTRAINT "service_requests_space_fkey" FOREIGN KEY ("space") REFERENCES "venue"."spaces"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "operations"."service_requests" ADD CONSTRAINT "service_requests_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "pricing"."seasons" ADD CONSTRAINT "seasons_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "pricing"."price_rules" ADD CONSTRAINT "price_rules_season_fkey" FOREIGN KEY ("season") REFERENCES "pricing"."seasons"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "pricing"."price_rules" ADD CONSTRAINT "price_rules_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."leads" ADD CONSTRAINT "leads_contact_fkey" FOREIGN KEY ("contact") REFERENCES "guest"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."leads" ADD CONSTRAINT "leads_venue_fkey" FOREIGN KEY ("venue") REFERENCES "venue"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."activities" ADD CONSTRAINT "activities_lead_id_fkey" FOREIGN KEY ("lead_id") REFERENCES "sales"."leads"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."quotes" ADD CONSTRAINT "quotes_contact_fkey" FOREIGN KEY ("contact") REFERENCES "guest"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."quotes" ADD CONSTRAINT "quotes_lead_fkey" FOREIGN KEY ("lead") REFERENCES "sales"."leads"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "sales"."quotes" ADD CONSTRAINT "quotes_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "stay"."reservations" ADD CONSTRAINT "reservations_contact_fkey" FOREIGN KEY ("contact") REFERENCES "guest"."contacts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "stay"."reservations" ADD CONSTRAINT "reservations_lead_fkey" FOREIGN KEY ("lead") REFERENCES "sales"."leads"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "stay"."reservations" ADD CONSTRAINT "reservations_event_fkey" FOREIGN KEY ("event") REFERENCES "event"."resource"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "stay"."reservations" ADD CONSTRAINT "reservations_quote_fkey" FOREIGN KEY ("quote") REFERENCES "sales"."quotes"("name") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "stay"."reservations" ADD CONSTRAINT "reservations_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "venue"."spaces" ADD CONSTRAINT "spaces_venue_id_fkey" FOREIGN KEY ("venue_id") REFERENCES "venue"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

