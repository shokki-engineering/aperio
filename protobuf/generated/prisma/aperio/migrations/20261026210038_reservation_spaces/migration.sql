-- CreateTable
CREATE TABLE "stay"."reservation_spaces" (
    "id" TEXT NOT NULL,
    "reservation_id" TEXT NOT NULL,
    "space_id" TEXT NOT NULL,
    "space_name" TEXT NOT NULL,

    CONSTRAINT "reservation_spaces_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "reservation_spaces_space_id_idx" ON "stay"."reservation_spaces"("space_id");

-- CreateIndex
CREATE UNIQUE INDEX "reservation_spaces_reservation_id_space_id_key" ON "stay"."reservation_spaces"("reservation_id", "space_id");

-- AddForeignKey
ALTER TABLE "stay"."reservation_spaces" ADD CONSTRAINT "reservation_spaces_reservation_id_fkey" FOREIGN KEY ("reservation_id") REFERENCES "stay"."reservations"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "stay"."reservation_spaces" ADD CONSTRAINT "reservation_spaces_space_id_fkey" FOREIGN KEY ("space_id") REFERENCES "venue"."spaces"("id") ON DELETE CASCADE ON UPDATE CASCADE;
