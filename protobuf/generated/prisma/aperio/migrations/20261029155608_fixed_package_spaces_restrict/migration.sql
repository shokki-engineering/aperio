-- DropForeignKey
ALTER TABLE "catalog"."fixed_package_spaces" DROP CONSTRAINT "fixed_package_spaces_space_id_fkey";

-- AddForeignKey
ALTER TABLE "catalog"."fixed_package_spaces" ADD CONSTRAINT "fixed_package_spaces_space_id_fkey" FOREIGN KEY ("space_id") REFERENCES "venue"."spaces"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
